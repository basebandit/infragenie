package main

import (
	"fmt"
	"strings"

	"github.com/basebandit/infragenie/internal/scanners"
	"github.com/spf13/cobra"
)

func scannersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scanners",
		Short: "Manage and inspect supported scanners",
	}
	cmd.AddCommand(scannersListCmd())
	return cmd
}

func scannersListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all supported scanners with availability and stack coverage",
		Example: `  infragenie scanners list`,
		Run: func(_ *cobra.Command, _ []string) {
			meta := registeredScanners()

			// column widths
			const (
				wName     = 12
				wCat      = 8
				wStacks   = 38
				wAvail    = 10
			)

			header := fmt.Sprintf("%-*s  %-*s  %-*s  %-*s  %s",
				wName, "NAME",
				wCat, "CATEGORY",
				wStacks, "STACKS",
				wAvail, "AVAILABLE",
				"INSTALL",
			)
			fmt.Println(header)
			fmt.Println(strings.Repeat("─", len(header)+10))

			for _, m := range meta {
				avail := "no"
				install := m.InstallHint
				if m.Scanner.Available() {
					avail = "yes"
					install = ""
				}

				stacks := ""
				if sa, ok := m.Scanner.(scanners.StackAware); ok {
					stacks = strings.Join(sa.Stacks(), ", ")
				} else {
					stacks = "all"
				}
				if len(stacks) > wStacks {
					stacks = stacks[:wStacks-1] + "…"
				}

				fmt.Printf("%-*s  %-*s  %-*s  %-*s  %s\n",
					wName, m.Scanner.Name(),
					wCat, m.Category,
					wStacks, stacks,
					wAvail, avail,
					install,
				)
			}
		},
	}
}
