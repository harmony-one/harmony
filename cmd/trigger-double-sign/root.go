package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"

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
	resp, err := http.Post(node, "application/json", bytes.NewBuffer(requestBody))
	fmt.Printf("URL: %s, Request Body: %s\n\n", node, string(requestBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	fmt.Printf("URL: %s, Response Body: %s\n\n", node, string(body))
	return body, err
}

var (
	rootCmd = &cobra.Command{
		Use:          "trigger-double-sign",
		SilenceUsage: true,
		Long:         "trigger a double sign",
		Run: func(cmd *cobra.Command, args []string) {
			baseRequest("http://localhost:7777/trigger-next-double-sign")
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
