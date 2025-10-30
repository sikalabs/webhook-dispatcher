package root

import (
	"github.com/sikalabs/webhook-dispatcher/version"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "webhook-dispatcher",
	Short: "webhook-dispatcher, " + version.Version,
}
