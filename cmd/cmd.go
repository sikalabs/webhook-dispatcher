package cmd

import (
	"github.com/sikalabs/webhook-dispatcher/cmd/root"
	_ "github.com/sikalabs/webhook-dispatcher/cmd/server"
	_ "github.com/sikalabs/webhook-dispatcher/cmd/version"
	"github.com/spf13/cobra"
)

func Execute() {
	cobra.CheckErr(root.Cmd.Execute())
}
