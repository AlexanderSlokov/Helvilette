package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"helvilette/pkg/playbook"
	"helvilette/pkg/storage"
)

var (
	port     int
	dataDir  string
	logLevel string
)

var rootCmd = &cobra.Command{
	Use:   "othela",
	Short: "Othela Control Plane for Helvilette",
	Long:  `Othela is the central control plane for the Helvilette OS Service Orchestration Framework.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Printf("Starting Othela with LogLevel: %s, DataDir: %s, Port: %d", logLevel, dataDir, port)

		addr := fmt.Sprintf(":%d", port)

		// Track resources that need cleanup on shutdown
		var closers []io.Closer

		cfg := ServerConfig{
			DebugMode: logLevel == "debug",
		}

		// Initialize SQLite storage at {data-dir}/server/db/state.db
		// following k3s convention for DB file location.
		dbPath := filepath.Join(dataDir, "server", "db", "state.db")
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

		// Initialize playbook loader
		loader, err := playbook.NewLoader(dataDir)
		if err != nil {
			log.Printf("[WARN] Could not initialize playbook loader at %s: %v", dataDir, err)
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
			log.Printf("Othela Control Plane is listening on %s...", addr)
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
		log.Printf("[SHUTDOWN] Marked server as not-ready, draining connections...")

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
	rootCmd.Flags().StringVarP(&dataDir, "data-dir", "d", "helvillette/othela/data/playbooks", "Directory to load playbooks from")
	rootCmd.Flags().StringVarP(&logLevel, "log-level", "l", "info", "Log level (debug, info, warn, error)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

