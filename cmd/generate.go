// Package cmd provides the command-line interface for nereus.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	outputFile string
	force      bool
)

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate sample configuration files",
	Long:  `Generate sample configuration files for nereus SNMP trap alerting system.`,
	Example: `# Generate config to stdout
	nereus generate

	# Generate config to specific file
	nereus generate --output config.yaml

	# Overwrite existing file
	nereus generate --output config.yaml --force`,
	RunE: generateConfig,
}

func init() {
	rootCmd.AddCommand(generateCmd)

	generateCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file path (default: stdout)")
	generateCmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing file")
}

func generateConfig(cmd *cobra.Command, args []string) error {
	// Create sample configuration YAML content
	configYAML := `# Nereus SNMP Trap Alerting System Configuration
# This is a sample configuration file with default values and examples.
# Modify the values according to your environment and requirements.

server:
  host: "0.0.0.0"
  port: 162
  community: "public"
  max_handlers: 100
  read_timeout: "30s"
  buffer_size: 8192

mibs:
  path: "/opt/mibs"
  auto_load: true
  recursive: true
  cache_enabled: true
  cache_path: "/tmp/nereus_mib_cache.json"

webhooks:
  - name: "alertmanager"
    url: "http://alertmanager:9093/api/v2/alerts"
    timeout: "30s"
    retry_count: 3
    retry_delay: "5s"
    enabled: true
    headers:
      Content-Type: "application/json"

  - name: "slack"
    url: "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK"
    timeout: "15s"
    retry_count: 2
    retry_delay: "3s"
    enabled: false
    filters:
      severity:
        - "critical"
        - "major"

logging:
  level: "info"
  format: "json"
  component: "nereus"
  stdout: true
  max_size: "100MB"
  max_backups: 5
  max_age: "30d"
`

	// Output to file or stdout
	if outputFile == "" {
		fmt.Print(configYAML)
		return nil
	}

	// Check if file exists and force flag
	if _, err := os.Stat(outputFile); err == nil && !force {
		return fmt.Errorf("file %s already exists, use --force to overwrite", outputFile)
	}

	// Create directory if needed
	if dir := filepath.Dir(outputFile); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Write to file
	if err := os.WriteFile(outputFile, []byte(configYAML), 0644); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}

	fmt.Printf("Configuration file generated: %s\n", outputFile)
	return nil
}

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file path (default: stdout)")
	generateCmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing file")
}
