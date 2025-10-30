package server

import (
	"github.com/sikalabs/webhook-dispatcher/cmd/root"
	"github.com/sikalabs/webhook-dispatcher/pkg/server"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:     "server",
	Short:   "Run server",
	Aliases: []string{"s"},
	Args:    cobra.NoArgs,
	Run: func(c *cobra.Command, args []string) {
		server.Server()
	},
}

func init() {
	root.Cmd.AddCommand(Cmd)
}
