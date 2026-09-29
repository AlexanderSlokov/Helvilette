package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"helvilette/pkg/log"
	"helvilette/pkg/storage"
)

var (
	port              int
	fleetRepo         string
	fleetBranch       string
	fleetSyncInterval time.Duration
	stateDir          string
	logLevel          string
)

// The pre-ADR-0003 flag. Intercepted rather than silently ignored: it used to
// designate the playbook directory while also receiving the SQLite state, so
// mapping it onto either replacement would be wrong half the time.
// See docs/informations/ADRs/ADR-0003.md.
const (
	// Removed flags that break existing invocations
	removedDataDirFlag      = "--data-dir"
	removedDataDirShorthand = "-d"
	removedPlaybookDirFlag  = "--playbook-dir"
)

const (
	// Writable state. FHS convention for variable state, which the systemd unit
	// files in BACKLOG 3.5 will need; k3s keeps its SQLite store under the same
	// scheme at /var/lib/rancher/k3s/server/db/state.db.
	defaultStateDir = "/var/lib/helvilette/othela"
)

var rootCmd = &cobra.Command{
	Use:   "othela",
	Short: "Control Plane of Helvilette",
	Long:  `Helvilette Othela is the control plane of Helvilette fleet.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetLevel(logLevel)
		logger := log.WithComponent("othela")

		if fleetRepo == "" {
			logger.Fatal().Str("flag", "--fleet-repo").Msg("required flag not set")
		}

		logger.Info().
			Str("log_level", logLevel).
			Str("fleet_repo", fleetRepo).
			Str("state_dir", stateDir).
			Int("port", port).
			Msg("starting othela")

		addr := fmt.Sprintf(":%d", port)

		// Track resources that need cleanup on shutdown
		var closers []io.Closer

		cfg := ServerConfig{}

		// State lives under --state-dir, not in --playbook-dir. Keeping the
		// database out of the playbook directory is what stops Othela writing
		// into a source tree; see ADR-0003.
		dbPath := filepath.Join(stateDir, "db", "state.db")
		sqliteStore, err := storage.NewSQLiteStore(dbPath)
		if err != nil {
			logger.Warn().Err(err).Str("db_path", dbPath).
				Msg("could not initialize SQLite, falling back to in-memory storage — check directory exists and is writable, or use --state-dir")
		} else {
			logger.Info().Str("db_path", dbPath).Msg("sqlite initialized")
			cfg.NodeStore = sqliteStore
			cfg.ReportStore = sqliteStore
			closers = append(closers, sqliteStore)
		}

		server := NewServerWithConfig(cfg)

		// Cancelled during drain so the poll loop does not outlive the server.
		syncCtx, stopFleetSync := context.WithCancel(context.Background())
		defer stopFleetSync()

		server.StartFleetSync(syncCtx, FleetSyncConfig{
			Repo:     fleetRepo,
			Branch:   fleetBranch,
			CacheDir: filepath.Join(stateDir, "fleet"),
			Interval: fleetSyncInterval,
		})

		httpServer := server.NewHTTPServer(addr)

		// Graceful shutdown: listen for SIGINT / SIGTERM
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		// Start serving in a goroutine
		errChan := make(chan error, 1)
		go func() {
			logger.Info().Str("addr", addr).Msg("othela listening")
			errChan <- httpServer.ListenAndServe()
		}()

		// Block until signal or server error
		select {
		case sig := <-sigChan:
			logger.Info().Str("signal", sig.String()).Msg("received shutdown signal")
		case err := <-errChan:
			logger.Fatal().Err(err).Msg("server failed unexpectedly")
		}

		// Mark server as not ready so readiness probes fail during drain
		server.SetReady(false)
		stopFleetSync()
		logger.Info().Msg("marked not-ready, draining connections")

		// Give in-flight requests time to complete
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(ctx); err != nil {
			logger.Fatal().Err(err).Msg("graceful shutdown failed")
		}

		// Close storage backends (SQLite, etc.)
		for _, c := range closers {
			if err := c.Close(); err != nil {
				logger.Error().Err(err).Msg("failed to close resource during shutdown")
			}
		}

		logger.Info().Msg("othela stopped gracefully")
	},
}

func init() {
	rootCmd.Flags().IntVarP(&port, "port", "p", 8080, "Port to listen on")
	rootCmd.Flags().StringVar(&fleetRepo, "fleet-repo", "", "Git repository containing Helvilette manifests (required)")
	rootCmd.Flags().StringVar(&fleetBranch, "fleet-branch", "main", "Branch of the fleet repository to sync")
	rootCmd.Flags().DurationVar(&fleetSyncInterval, "fleet-sync-interval", 1*time.Minute, "Interval to sync the fleet repository")
	rootCmd.Flags().StringVar(&stateDir, "state-dir", defaultStateDir,
		"Directory for writable state (SQLite database, caches)")
	rootCmd.Flags().StringVarP(&logLevel, "log-level", "l", "info", "Log level (debug, info, warn, error)")
}

// removedFlagError reports the removal of --data-dir with the replacement flags
// named. Cobra's default "unknown flag" text would leave the operator guessing,
// and the old flag has no single correct successor: it designated the playbook
// directory while also receiving state, so it maps onto both new flags at once.
//
// Usage:
//
//	if err := removedFlagError(os.Args[1:]); err != nil { ... }
func removedFlagError(args []string) error {
	for _, arg := range args {
		if strings.HasPrefix(arg, removedPlaybookDirFlag) {
			return fmt.Errorf(
				"%s was removed. Othela now resolves playbooks by Git reference. Use --fleet-repo to specify the Git repository containing manifests.",
				removedPlaybookDirFlag)
		}
		if !isRemovedDataDirArg(arg) {
			continue
		}
		return fmt.Errorf(
			"%s was removed: it named the playbook directory but also received the SQLite state at "+
				"{data-dir}/server/db/state.db. Use --fleet-repo for GitOps manifests "+
				"and --state-dir for writable state (default %q).",
			removedDataDirFlag, defaultStateDir)
	}
	return nil
}

func isRemovedDataDirArg(arg string) bool {
	return arg == removedDataDirFlag ||
		strings.HasPrefix(arg, removedDataDirFlag+"=") ||
		arg == removedDataDirShorthand ||
		strings.HasPrefix(arg, removedDataDirShorthand+"=")
}

func main() {
	if err := removedFlagError(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
