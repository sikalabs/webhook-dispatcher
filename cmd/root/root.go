package root

import (
	"github.com/sikalabs/webhook-to-redis/version"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "webhook-to-redis",
	Short: "webhook-to-redis, " + version.Version,
}
