package cmd

import (
	"fmt"
	"github.com/amin1024/xtelbot/core"
	"github.com/amin1024/xtelbot/core/repo/models"
	"github.com/amin1024/xtelbot/telbot"
	"github.com/spf13/cobra"
)

var botCmd = &cobra.Command{
	Use:   "bot",
	Short: "Start bot",

	Run: func(cmd *cobra.Command, args []string) {
		bh := telbot.NewBotHandler()
		bh.Start()
	},
}

var addXNodeCmd = &cobra.Command{
	Use:   "add-xnode",
	Short: "introduce a new xnode(aka panel) to the bot",

	Run: func(cmd *cobra.Command, args []string) {
		// TODO: refactor this shit
		if len(args) != 2 {
			fmt.Println("Invalid arguments. Please specify a valid address (host:port) and a panel type.")
			return
		}
		node := models.Xnode{
			Address:   args[0],
			PanelType: args[1],
		}
		if err := core.AddXNode(&node); err != nil {
			fmt.Println("failed. " + err.Error())
			return
		}
		fmt.Println("XNode added.")
	},
}

func init() {
	botCmd.AddCommand(addXNodeCmd)
}
