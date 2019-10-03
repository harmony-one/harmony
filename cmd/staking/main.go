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
	if os.Args[1] == "-version" {
		fmt.Fprintf(os.Stderr, versionS()+"\n")
		os.Exit(0)
	}
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
