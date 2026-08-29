package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"helvilette/pkg/playbook"
	"helvilette/pkg/storage"
)

var (
	port        int
	playbookDir string
	stateDir    string
	logLevel    string
)

// The pre-ADR-0003 flag. Intercepted rather than silently ignored: it used to
// designate the playbook directory while also receiving the SQLite state, so
// mapping it onto either replacement would be wrong half the time.
// See docs/informations/ADRs/ADR-0003.md.
const (
	removedDataDirFlag      = "--data-dir"
	removedDataDirShorthand = "-d"
)

const (
	// Read-only playbook input. Relative so a development checkout works from
	// the repository root without arguments.
	defaultPlaybookDir = "helvilette/othela/data/playbooks"

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
		log.Printf("Starting Helvilette Othela with LogLevel: %s, PlaybookDir: %s, StateDir: %s, Port: %d",
			logLevel, playbookDir, stateDir, port)

		addr := fmt.Sprintf(":%d", port)

		// Track resources that need cleanup on shutdown
		var closers []io.Closer

		cfg := ServerConfig{
			DebugMode: logLevel == "debug",
		}

		// State lives under --state-dir, not in --playbook-dir. Keeping the
		// database out of the playbook directory is what stops Othela writing
		// into a source tree; see ADR-0003.
		dbPath := filepath.Join(stateDir, "db", "state.db")
		sqliteStore, err := storage.NewSQLiteStore(dbPath)
		if err != nil {
			log.Printf("[WARN] Could not initialize SQLite at %s: %v", dbPath, err)
			log.Printf("[WARN] Falling back to in-memory storage")
		} else {
			log.Printf("[STORAGE] SQLite initialized at %s", dbPath)
			cfg.NodeStore = sqliteStore
			cfg.ReportStore = sqliteStore
			closers = append(closers, sqliteStore)
		}

		// Playbook directory is read-only input. Othela never writes here.
		loader, err := playbook.NewLoader(playbookDir)
		if err != nil {
			log.Printf("[WARN] Could not initialize playbook loader at %s: %v", playbookDir, err)
			log.Printf("[WARN] Starting without playbook loading")
		} else {
			cfg.Loader = loader
		}

		server := NewServerWithConfig(cfg)

		httpServer := server.NewHTTPServer(addr)

		// Graceful shutdown: listen for SIGINT / SIGTERM
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		// Start serving in a goroutine
		errChan := make(chan error, 1)
		go func() {
			log.Printf("Helvilette Othela is listening on %s...", addr)
			errChan <- httpServer.ListenAndServe()
		}()

		// Block until signal or server error
		select {
		case sig := <-sigChan:
			log.Printf("[SHUTDOWN] Received signal: %v", sig)
		case err := <-errChan:
			log.Fatalf("Server failed unexpectedly: %v", err)
		}

		// Mark server as not ready so readiness probes fail during drain
		server.SetReady(false)
		log.Printf("[SHUTDOWN] Marked this Helvilette Othela as not-ready, draining connections...")

		// Give in-flight requests time to complete
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(ctx); err != nil {
			log.Fatalf("[SHUTDOWN] Graceful shutdown failed: %v", err)
		}

		// Close storage backends (SQLite, etc.)
		for _, c := range closers {
			if err := c.Close(); err != nil {
				log.Printf("[SHUTDOWN] Error closing resource: %v", err)
			}
		}

		log.Printf("[SHUTDOWN] Othela stopped gracefully.")
	},
}

func init() {
	rootCmd.Flags().IntVarP(&port, "port", "p", 8080, "Port to listen on")
	rootCmd.Flags().StringVar(&playbookDir, "playbook-dir", defaultPlaybookDir,
		"Directory to load playbooks from. Read-only; Othela never writes here")
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
		if !isRemovedDataDirArg(arg) {
			continue
		}
		return fmt.Errorf(
			"%s was removed: it named the playbook directory but also received the SQLite state at "+
				"{data-dir}/server/db/state.db. Use --playbook-dir for read-only playbooks (default %q) "+
				"and --state-dir for writable state (default %q).",
			removedDataDirFlag, defaultPlaybookDir, defaultStateDir)
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
