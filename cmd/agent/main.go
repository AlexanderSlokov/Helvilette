// Command agent is the Helvilette node agent. It polls Othela for work, runs the
// Ansible playbook the job names, and reports the result back.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"helvilette/pkg/log"
)

// cliFlags holds the raw flag values, before LoadConfig layers them over the
// config file, the environment and the defaults. Empty means "not given", which
// is what lets a lower-precedence source win. See ADR-0001.
type cliFlags struct {
	configFile   string
	othelaURL    string
	nodeID       string
	pollInterval string
	workspaceDir string
	labels       string
	printConfig  bool
}

// newRootCmd builds the agent's command with every flag registered.
//
// Separated from main so a test can reach the flag set. The agent's systemd unit
// passes flags that have to exist, and nothing checked that until this could be
// called from a test; see BACKLOG 6.7 and the Othela equivalent in
// TestOthelaUnitFlagsExistOnTheCLI.
//
//	if cmd := newRootCmd(); cmd.Flags().Lookup("node-id") == nil { ... }
func newRootCmd() *cobra.Command {
	var flags cliFlags

	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Helvilette Node Agent",
		Long:  `The Node Agent runs on client machines, polls Othela for jobs, and executes Ansible playbooks.`,
		Run: func(_ *cobra.Command, _ []string) {
			runAgent(flags)
		},
	}

	registerFlags(cmd, &flags)
	return cmd
}

func registerFlags(cmd *cobra.Command, flags *cliFlags) {
	f := cmd.Flags()
	f.StringVar(&flags.configFile, "config", "", "Path to the YAML configuration file (e.g. /var/lib/helvilette/agent.yaml)")
	f.StringVar(&flags.othelaURL, "othela-url", "", "URL of the Othela control plane")
	f.StringVar(&flags.nodeID, "node-id", "", "Unique identifier for this node")
	f.StringVar(&flags.pollInterval, "poll-interval", "", "Interval between polls to Othela (e.g. 5s)")
	f.StringVar(&flags.workspaceDir, "workspace-dir", "", "Directory for storing agent workspace files")
	f.StringVar(&flags.labels, "labels", "", "Comma-separated key=value labels (e.g. role=web,env=prod)")
	f.BoolVar(&flags.printConfig, "print-config", false, "Print the resolved configuration and the source of each value, then exit")
}

// runAgent resolves the configuration and runs until the process is signalled.
func runAgent(flags cliFlags) {
	cfg, provenance, err := LoadConfig(flags.configFile, flags.othelaURL, flags.nodeID,
		flags.pollInterval, flags.workspaceDir, flags.labels)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load configuration")
	}

	// Resolve and print without starting, so a config can be verified during
	// day-0 bring-up or in CI rather than by observing a running agent.
	if flags.printConfig {
		fmt.Print(FormatConfig(cfg, provenance))
		return
	}

	logEffectiveConfig(cfg, provenance)

	ctx, cancel := signalledContext()
	defer cancel()

	if err := NewAgent(cfg).Run(ctx); err != nil {
		log.Fatal().Err(err).Msg("agent stopped with error")
	}
}

// signalledContext returns a context cancelled on SIGINT or SIGTERM, so a
// shutdown lands between polls rather than mid-playbook.
func signalledContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Info().Str("signal", sig.String()).Msg("received shutdown signal")
		cancel()
	}()

	return ctx, cancel
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
