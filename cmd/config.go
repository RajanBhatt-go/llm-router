package cmd

import (
	"fmt"
	"os"

	"llm-router/internal/config"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a default config file",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		configDir := home + "/.config/llm-router"
		configPath := configDir + "/llm-router.yaml"

		if err := os.MkdirAll(configDir, 0755); err != nil {
			return err
		}

		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("config already exists: %s", configPath)
		}

		if err := config.InitConfigFile(configPath); err != nil {
			return fmt.Errorf("write config: %w", err)
		}

		fmt.Printf("Config created: %s\n", configPath)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configInitCmd)
	rootCmd.AddCommand(configCmd)
}