package main

import (
	"fmt"
	"os"

	gen "github.com/basebandit/infragenie/internal/generate"
	"github.com/basebandit/infragenie/internal/goldenpath"
	"github.com/basebandit/infragenie/pkg/models"
	"github.com/spf13/cobra"
)

func generateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Scaffold new resources that conform to your Golden Path",
		Long: `Generate produces resources that are correct from day one — rendered
directly from your goldenpath.yml so they pass infragenie review with zero
Golden Path findings.`,
	}
	cmd.AddCommand(generateServiceCmd())
	return cmd
}

func generateServiceCmd() *cobra.Command {
	var (
		template     string
		outPath      string
		gpPath       string
		force        bool
		listTemplate bool
	)

	cmd := &cobra.Command{
		Use:   "service <name>",
		Short: "Scaffold a new service",
		Example: `  infragenie generate service my-api
  infragenie generate service my-api --template k8s-service --goldenpath goldenpath.yml
  infragenie generate service --list-templates`,
		RunE: func(_ *cobra.Command, args []string) error {
			if listTemplate {
				for _, n := range gen.TemplateNames() {
					fmt.Printf("%-16s %s\n", n, gen.Describe(n))
				}
				return nil
			}
			if len(args) != 1 {
				return fmt.Errorf("exactly one service name is required")
			}
			name := args[0]

			gp, err := resolveGoldenPath(gpPath)
			if err != nil {
				return err
			}

			res, err := gen.New().Run(gen.Params{
				Name:       name,
				Template:   template,
				OutDir:     outPath,
				GoldenPath: gp,
				Force:      force,
			})
			if err != nil {
				return err
			}

			fmt.Printf("scaffolded %q (%s) in %s\n", name, res.Template, res.Dir)
			for _, f := range res.Files {
				fmt.Printf("  %s\n", f)
			}
			fmt.Printf("next: infragenie review --goldenpath %s --path %s\n", gpFlagOrDefault(gpPath), res.Dir)
			return nil
		},
	}

	cmd.Flags().StringVarP(&template, "template", "t", gen.DefaultTemplate, "template set to render")
	cmd.Flags().StringVarP(&outPath, "path", "p", "services", "parent directory for the generated service")
	cmd.Flags().StringVarP(&gpPath, "goldenpath", "g", "", "path to goldenpath.yml (default: ./goldenpath.yml if present)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing files")
	cmd.Flags().BoolVar(&listTemplate, "list-templates", false, "list available templates and exit")
	return cmd
}

// resolveGoldenPath loads the Golden Path from gpPath, falling back to
// ./goldenpath.yml when present. A nil result is valid — the generator uses
// secure defaults.
func resolveGoldenPath(gpPath string) (*models.GoldenPath, error) {
	path := gpPath
	if path == "" {
		if _, err := os.Stat("goldenpath.yml"); err == nil {
			path = "goldenpath.yml"
		}
	}
	if path == "" {
		return nil, nil
	}
	gp, err := goldenpath.New(".").Load(path)
	if err != nil {
		return nil, fmt.Errorf("goldenpath: %w", err)
	}
	return gp, nil
}

func gpFlagOrDefault(gpPath string) string {
	if gpPath != "" {
		return gpPath
	}
	return "goldenpath.yml"
}
