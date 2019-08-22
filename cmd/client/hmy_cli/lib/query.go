package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/spf13/cobra"
)

const (
	JSON_RPC_VERSION = "2.0"
)

var (
	node     string
	queryID  = 0
	cmdQuery = &cobra.Command{
		Use:   "account",
		Short: "Query account balance",
		Long:  `Query account balances`,
		Run: func(cmd *cobra.Command, args []string) {
			requestBody, _ := json.Marshal(map[string]interface{}{
				"jsonrpc": JSON_RPC_VERSION,
				"id":      strconv.Itoa(queryID),
				"method":  "hmy_getBalance",
				"params":  [...]string{"0xD7Ff41CA29306122185A07d04293DdB35F24Cf2d", "latest"},
			})

			resp, err := http.Post(node, "application/json", bytes.NewBuffer(requestBody))
			if err != nil {
				fmt.Println(err)
			}

			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				fmt.Println("error")
			}
			fmt.Println(string(body))
		},
	}
)

func init() {
	cmdQuery.Flags().StringVarP(
		&node,
		"node",
		"",
		"http://localhost:9500",
		"<host>:<port>",
	)
	rootCmd.AddCommand(cmdQuery)
}
