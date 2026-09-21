package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "llm-router",
	Short: "Intelligent LLM request router with local fallback",
	Long: `llm-router accepts LLM execution requests, attempts OpenRouter
(DeepSeek V4 Flash), and gracefully falls back to a local model (Ollama)
if the cloud times out or errors.

It standardizes context window truncation and tool-calling across providers.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default $HOME/.config/llm-router/llm-router.yaml)")
}