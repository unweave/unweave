package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/unweave/unweave/cli/config"
)

func Logout(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	config.Config.User.Token = ""
	config.Config.Save()
	fmt.Println("Logged out")
	return nil
}