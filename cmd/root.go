// Package cmd provides the command-line interface for nereus.

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/geekxflood/common/config"
	"github.com/geekxflood/nereus/internal/snmp"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	version = "dev" // Will be set by build flags
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "nereus",
	Version: version,
	Short:   "Advanced SNMP trap alerting system",
	Long: `Nereus is an advanced SNMP trap alerting system designed to monitor, parse,
and manage alerts with intelligent event correlation and notification capabilities.`,
	Example: `# Start the SNMP trap listener with default config
	nereus

	# Start with specific configuration file
	nereus --config /etc/nereus/config.yaml

	# Generate sample configuration
	nereus generate --output config.yaml

	# Validate configuration
	nereus validate --config config.yaml`,
	RunE: runServer,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func runServer(cmd *cobra.Command, args []string) error {
	// Load configuration
	manager, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	defer manager.Close()

	// Get configuration values
	host, _ := manager.GetString("server.host", "0.0.0.0")
	port, _ := manager.GetInt("server.port", 162)
	mibPath, _ := manager.GetString("mibs.path", "/opt/mibs")

	fmt.Printf("Starting nereus SNMP trap alerting system...\n")
	fmt.Printf("Listening on %s:%d\n", host, port)
	fmt.Printf("MIB path: %s\n", mibPath)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nReceived shutdown signal, stopping server...")
		cancel()
	}()

	// Initialize and start SNMP trap listener
	listener, err := snmp.NewListener(manager)
	if err != nil {
		return fmt.Errorf("failed to create SNMP listener: %w", err)
	}

	if err := listener.Start(ctx); err != nil {
		return fmt.Errorf("failed to start SNMP listener: %w", err)
	}

	fmt.Println("SNMP trap listener started successfully")

	// TODO: Initialize MIB parser
	// TODO: Initialize webhook notifiers
	// TODO: Start event correlation engine

	fmt.Println("Server started successfully. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	<-ctx.Done()

	// Stop SNMP listener
	fmt.Println("Stopping SNMP listener...")
	if err := listener.Stop(); err != nil {
		fmt.Printf("Error stopping SNMP listener: %v\n", err)
	}

	fmt.Println("Server stopped.")

	return nil
}

func loadConfig() (config.Manager, error) {
	// Determine config file path
	configPath := cfgFile
	if configPath == "" {
		// Try default locations
		defaultPaths := []string{
			"config.yaml",
			"config.yml",
			"/etc/nereus/config.yaml",
			"/etc/nereus/config.yml",
		}

		for _, path := range defaultPaths {
			if _, err := os.Stat(path); err == nil {
				configPath = path
				break
			}
		}
	}

	// Create config manager options
	options := config.Options{
		SchemaPath: "cmd/schemas/config.cue",
		ConfigPath: configPath,
	}

	if configPath == "" {
		fmt.Println("No configuration file found, using schema defaults")
	} else {
		fmt.Printf("Loading configuration from: %s\n", configPath)
	}

	// Create config manager
	manager, err := config.NewManager(options)
	if err != nil {
		return nil, fmt.Errorf("failed to create config manager: %w", err)
	}

	return manager, nil
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "Configuration file path")

	// Handle version flag
	rootCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if versionFlag, _ := cmd.Flags().GetBool("version"); versionFlag {
			fmt.Printf("nereus version %s\n", version)
			os.Exit(0)
		}
		return nil
	}
}
