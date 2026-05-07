package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "infragenie",
		Short: "Golden Path enforcement — review PRs and generate conformant services",
		Long: `infragenie enforces your engineering standards.

Codify standards once in goldenpath.yml, then:
  review  — scan pull requests and local diffs against your Golden Path
  generate — scaffold new services that are correct from day one`,
		SilenceUsage: true,
	}
	root.AddCommand(initCmd())
	root.AddCommand(reviewCmd())
	root.AddCommand(mcpCmd())
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run:   func(_ *cobra.Command, _ []string) { fmt.Println(version) },
	})
	return root
}
