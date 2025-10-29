package cmd

import (
	"github.com/sikalabs/webhook-to-redis/cmd/root"
	_ "github.com/sikalabs/webhook-to-redis/cmd/version"
	"github.com/spf13/cobra"
)

func Execute() {
	cobra.CheckErr(root.Cmd.Execute())
}
