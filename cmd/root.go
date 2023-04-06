package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

const version = 0.1

var log *zap.SugaredLogger

func init() {
	l, _ := zap.NewProduction()
	log = l.Sugar()

	rootCmd.AddCommand(versionCmd, botCmd, panelCmd)
}

var rootCmd = &cobra.Command{
	Use:   "xtelbot",
	Short: "xTelBot helps you to manage your users through telegram-bot and xray-panels",
	Run: func(cmd *cobra.Command, args []string) {
		log.Info("NO OP")
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Perhaps we've never been visited by aliens because they have looked upon"+
			"Earth and decided there's no sign of intelligent life.\n-Neil deGrasse Tyson\n\n"+
			"version: v%v\n", version)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}
