package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"

	"helvilette/pkg/playbook"
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

		loader, err := playbook.NewLoader(dataDir)
		if err != nil {
			log.Printf("[WARN] Could not initialize playbook loader at %s: %v", dataDir, err)
			log.Printf("[WARN] Falling back to default server (no playbook loading)")
			server := NewServer()
			if err := server.ListenAndServe(addr); err != nil {
				log.Fatalf("Server failed: %v", err)
			}
			return
		}

		server := NewServerWithLoader(loader)
		if err := server.ListenAndServe(addr); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
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
