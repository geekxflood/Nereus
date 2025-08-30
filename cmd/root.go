// Package cmd provides the command-line interface for nereus.

package cmd

import (
	"fmt"
	"os"

	"github.com/geekxflood/common/config"
	"github.com/geekxflood/nereus/internal/app"
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
	configManager, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	defer configManager.Close()

	fmt.Printf("Starting nereus SNMP trap alerting system...\n")

	// Create application
	application, err := app.NewApplication(configManager)
	if err != nil {
		return fmt.Errorf("failed to create application: %w", err)
	}

	// Initialize all components
	if err := application.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize application: %w", err)
	}

	fmt.Println("All components initialized successfully")

	// Run application (blocks until shutdown)
	if err := application.Run(); err != nil {
		return fmt.Errorf("application error: %w", err)
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
