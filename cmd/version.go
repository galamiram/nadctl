package cmd

import (
	"fmt"

	"github.com/galamiram/nadctl/internal/version"
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display the current version of NAD Controller.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Display version information
		fmt.Printf("NAD Controller version: %s\n", version.Version)
		fmt.Printf("Terminal User Interface for Premium Audio Control\n")
		fmt.Printf("https://github.com/galamiram/nadctl\n")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
