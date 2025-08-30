// Package cmd provides the command-line interface for nereus.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/geekxflood/common/config"
	"github.com/spf13/cobra"
)

var (
	checkMIBs bool
)

// validateCmd represents the validate command
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration and MIB files",
	Long:  `Validate configuration files and optionally check MIB file accessibility.`,
	Example: `# Validate configuration file
	nereus validate --config config.yaml

	# Validate configuration and check MIB files
	nereus validate --config config.yaml --check-mibs

	# Validate using default config locations
	nereus validate`,
	RunE: validateConfig,
}

func init() {
	rootCmd.AddCommand(validateCmd)

	validateCmd.Flags().BoolVar(&checkMIBs, "check-mibs", false, "Also validate MIB file accessibility")
}

func validateConfig(cmd *cobra.Command, args []string) error {
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

		if configPath == "" {
			return fmt.Errorf("no configuration file found, specify with --config or create config.yaml")
		}
	}

	fmt.Printf("Validating configuration file: %s\n", configPath)

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("configuration file not found: %s", configPath)
	}

	// Create config manager to validate the configuration
	manager, err := config.NewManager(config.Options{
		SchemaPath: "cmd/schemas/config.cue",
		ConfigPath: configPath,
	})
	if err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}
	defer manager.Close()

	// Validate the configuration
	if err := manager.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	fmt.Println("✓ Configuration syntax is valid")

	// Validate MIB files if requested
	if checkMIBs {
		if err := validateMIBFiles(manager); err != nil {
			return fmt.Errorf("MIB validation failed: %w", err)
		}
		fmt.Println("✓ MIB files are accessible")
	}

	fmt.Println("✓ Configuration validation completed successfully")
	return nil
}

func validateMIBFiles(manager config.Provider) error {
	// Get MIB path from configuration
	mibPath, err := manager.GetString("mibs.path")
	if err != nil {
		return fmt.Errorf("mibs.path not found in configuration: %w", err)
	}

	// Check if MIB directory exists
	if _, err := os.Stat(mibPath); os.IsNotExist(err) {
		return fmt.Errorf("MIB directory does not exist: %s", mibPath)
	}

	// Check if directory is readable
	dir, err := os.Open(mibPath)
	if err != nil {
		return fmt.Errorf("cannot read MIB directory: %w", err)
	}
	defer dir.Close()

	// Count MIB files
	var mibCount int
	err = filepath.Walk(mibPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && (strings.HasSuffix(strings.ToLower(path), ".mib") ||
			strings.HasSuffix(strings.ToLower(path), ".txt")) {
			mibCount++
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("error scanning MIB directory: %w", err)
	}

	if mibCount == 0 {
		return fmt.Errorf("no MIB files found in directory: %s", mibPath)
	}

	fmt.Printf("  Found %d MIB files in %s\n", mibCount, mibPath)
	return nil
}
