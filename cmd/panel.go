package cmd

import (
	"github.com/amin1024/xtelbot/xpanels"
	"github.com/spf13/cobra"
)

var panelType string
var panelName string
var dbPath string
var resetScript string
var port string
var xrayPort string

var pargs = make(map[string]string)

var panelCmd = &cobra.Command{
	Use:   "panel",
	Short: "Start panel manager",

	Run: func(cmd *cobra.Command, args []string) {
		pargs["type"] = panelType
		pargs["name"] = panelName
		pargs["db"] = dbPath
		pargs["resetScript"] = resetScript
		pargs["port"] = port
		pargs["xrayPort"] = xrayPort

		xpanels.StartXPanel(pargs)
	},
}

func init() {
	panelCmd.PersistentFlags().StringVar(&panelType, "type", "", "specify the type of panel. Options are [xui, hiddify]")
	panelCmd.PersistentFlags().StringVar(&panelName, "name", "", "specify the panel name. default: generates a name based on host IP")
	panelCmd.PersistentFlags().StringVar(&dbPath, "db", "", "specify db path")
	panelCmd.PersistentFlags().StringVar(&resetScript, "", "", "TBD")
	panelCmd.PersistentFlags().StringVar(&port, "port", "7777", "port to be used by panel manager")
	panelCmd.PersistentFlags().StringVar(&xrayPort, "xray-port", "10085", "port to connect to xray-core")
	panelCmd.MarkPersistentFlagRequired("type")
	panelCmd.MarkPersistentFlagRequired("db")
	panelCmd.MarkPersistentFlagRequired("xray-port")
}
