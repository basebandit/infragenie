package main

import (
	"fmt"
	"os"

	"github.com/basebandit/infragenie/pkg/config"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	var configPath string
	var appCfg *config.AppConfig

	root := &cobra.Command{
		Use:   "infragenie",
		Short: "Golden Path enforcement — review PRs and generate conformant services",
		Long: `infragenie enforces your engineering standards.

Codify standards once in goldenpath.yml, then:
  review  — scan pull requests and local diffs against your Golden Path
  generate — scaffold new services that are correct from day one`,
		SilenceUsage: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			appCfg = cfg
			return nil
		},
	}

	root.PersistentFlags().StringVar(&configPath, "config", "",
		"config file (default: .infragenie.yml or ~/.config/infragenie/config.yml)")

	root.AddCommand(initCmd())
	root.AddCommand(reviewCmd(&appCfg))
	root.AddCommand(mcpCmd())
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run:   func(_ *cobra.Command, _ []string) { fmt.Println(version) },
	})
	return root
}
