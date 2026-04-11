package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	port    int
	dataDir string
	logLevel string
)

var rootCmd = &cobra.Command{
	Use:   "othela",
	Short: "Othela is the control plane for Helvilette",
	Long:  `A longer description that spans multiple lines and likely contains examples and usage of using your application.`,
	Run: func(cmd *cobra.Command, args []string) {
		// This is where the server startup logic will go
		fmt.Printf("Starting Othela on port %d\n", port)
		fmt.Printf("Data directory: %s\n", dataDir)
		fmt.Printf("Log level: %s\n", logLevel)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().IntVarP(&port, "port", "p", 8080, "Port to run the server on")
	rootCmd.PersistentFlags().StringVarP(&dataDir, "data-dir", "d", "/data", "Directory to store data")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "info", "Log level (debug, info, warn, error)")
}
