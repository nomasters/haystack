package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "haystack",
	Short: "Haystack is an ephemeral key value store",
	Long:  `Haystack is an ephemeral key value store`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("hello world")
	},
}

// Execute is the primary command used by haystack
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
