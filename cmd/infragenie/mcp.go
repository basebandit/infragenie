package main

import (
	"context"
	"os"

	infrageniemcp "github.com/basebandit/infragenie/internal/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

func mcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start the MCP server (stdio transport)",
		Long: `Start InfraGenie as an MCP server.

Add to your MCP client config:
  {
    "mcpServers": {
      "infragenie": {
        "command": "infragenie",
        "args": ["mcp"]
      }
    }
  }`,
		RunE: func(_ *cobra.Command, _ []string) error {
			s := infrageniemcp.NewServer(version)
			stdio := server.NewStdioServer(s)
			return stdio.Listen(context.Background(), os.Stdin, os.Stdout)
		},
	}
}
