package cmd

import (
	"fmt"
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

var addRenovateRule = &cobra.Command{
	Use:     "add-rule",
	Short:   "add a new renovate rule",
	Example: "xtelbot panel --db /tmp/db.sqlite add-rule remark host=old.host.com host=new.host.com",

	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 3 {
			fmt.Println("wrong number of arguments.")
			return
		}
		repo := xpanels.SetupHiddifyRepo(dbPath)
		r := xpanels.RenovateRule{
			Remark:   args[0],
			OldValue: args[1],
			NewValue: args[2],
		}
		// Usage command to ignore a rule: "./xtelbot add-rule #remark - -"
		if args[1] == "-" {
			r.Ignore = true
		}
		if err := repo.InsertRenovateRule(r); err != nil {
			fmt.Println(err.Error())
			return
		}
		fmt.Println("Done.")
	},
}

func init() {
	panelCmd.AddCommand(addRenovateRule)
	panelCmd.PersistentFlags().StringVar(&dbPath, "db", "", "specify db path")

	panelCmd.Flags().StringVar(&panelType, "type", "", "specify the panel type. Options are [xui, hiddify]")
	panelCmd.Flags().StringVar(&panelName, "name", "", "specify the panel name. default: generates a name based on host IP")
	panelCmd.PersistentFlags().StringVar(&resetScript, "", "", "TBD")
	panelCmd.PersistentFlags().StringVar(&port, "port", "7777", "port to be used by panel manager")
	panelCmd.Flags().StringVar(&xrayPort, "xray-port", "10085", "port to connect to xray-core")
	panelCmd.MarkPersistentFlagRequired("type")
	panelCmd.MarkPersistentFlagRequired("db")
	panelCmd.MarkPersistentFlagRequired("xray-port")
}
