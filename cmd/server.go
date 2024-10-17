package cmd

import (
	"fmt"

	"github.com/nomasters/haystack/x/udp/server"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.Flags().StringP("port", "p", "1337", "Port for the server listener")
	serverCmd.Flags().StringP("host", "", "", "hostname of server listener")
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run haystack in server mode.",
	Long:  `Server mode is used to run long-lived haystack servers.`,
	Run: func(cmd *cobra.Command, args []string) {
		opts := []server.Option{}
		port, _ := cmd.Flags().GetString("port")
		host, _ := cmd.Flags().GetString("host")
		addr := host + ":" + port
		fmt.Println("listening on:", addr)
		if err := server.ListenAndServe(addr, opts...); err != nil {
			fmt.Println(err)
		}
	},
}
