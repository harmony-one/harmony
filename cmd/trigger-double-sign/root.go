package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/harmony-one/harmony/staking/slash"
	"github.com/spf13/cobra"
)

func versionS() string {
	return fmt.Sprintf(
		"Harmony (C) 2019. %v, version %v-%v (%v %v)",
		path.Base(os.Args[0]), version, commit, builtBy, builtAt,
	)
}

func baseRequest(node string) ([]byte, error) {
	requestBody, _ := json.Marshal(map[string]interface{}{
		"test": "payload",
	})

	fmt.Printf(
		"Sent %s URL: %s, Request Body: %s\n\n",
		time.Now().Format(time.RFC3339),
		node,
		string(requestBody),
	)

	resp, err := http.Post(node, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	result := slash.ReportResult{}

	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Printf("error case but still raw dump URL: %s, Response Body:%s\n\n", node, string(body))
		return nil, err
	}

	fmt.Println(result)
	return body, err
}

var (
	p       = slash.DefaultWebHookPath
	rootCmd = &cobra.Command{
		Use:          "trigger-double-sign",
		SilenceUsage: true,
		Long:         "trigger a double sign",
		Run: func(cmd *cobra.Command, args []string) {
			config, err := slash.NewDoubleSignWebHooksFromPath(p)
			if err != nil {
				fmt.Fprintf(
					os.Stderr, "ERROR provided yaml path %s but yaml found is illegal", p,
				)
				os.Exit(1)
			}
			baseRequest(config.Malicious.Trigger.DoubleSignNodeURL)
		},
	}
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Show version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(os.Stderr, versionS()+"\n")
			os.Exit(0)
		},
	})
}
