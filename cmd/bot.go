package cmd

import (
	"fmt"
	"github.com/amin1024/xtelbot/api"
	"github.com/amin1024/xtelbot/core"
	"github.com/amin1024/xtelbot/core/repo/models"
	"github.com/amin1024/xtelbot/telbot"
	"github.com/spf13/cobra"
	"log"
	"net/url"
	"os"
)

var certFilePath string
var keyFilePath string
var domain string
var callbackUrl string

var botCmd = &cobra.Command{
	Use:   "bot",
	Short: "Start bot",

	Run: func(cmd *cobra.Command, args []string) {
		nextPayApiKey := os.Getenv("NEXTPAY_API_KEY")
		if nextPayApiKey == "" {
			log.Fatal("api_key required")
		}
		callbackPath := "npcallback"
		if callbackUrl == "" {
			callbackUrl, _ = url.JoinPath("https://", domain, callbackPath)
		}
		terminal := core.NewNextPayTerminal(nextPayApiKey, callbackUrl)

		userService := core.NewUserService(terminal)

		bh := telbot.NewBotHandler(userService, domain)
		restHandler := api.NewRestHandler(userService, callbackPath)
		go restHandler.Start(certFilePath, keyFilePath)
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

	// A certificate needed to listen on 443 port
	botCmd.Flags().StringVar(&certFilePath, "cert", "fullchain.pem", "specify the path to cert file")
	botCmd.Flags().StringVar(&keyFilePath, "key", "privkey.pem", "specify the path to key file")
	botCmd.Flags().StringVar(&domain, "domain", "", "specify the domain address (which is certified by the --cert file) to serve the sub-link")
	botCmd.Flags().StringVar(&callbackUrl, "callback", "", "specify the callback url address to be used by nextpay.org. Automatically set based on --domain argument")
	botCmd.MarkFlagRequired("cert")
	botCmd.MarkFlagRequired("key")
	botCmd.MarkFlagRequired("domain")
}
