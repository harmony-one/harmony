package main

import (
	"fmt"
	"os"
)

var (
	version string
	commit  string
	builtAt string
	builtBy string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
