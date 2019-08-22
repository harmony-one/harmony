package main

import (
	cmd "github.com/harmony-one/harmony/cmd/client/hmy_cli/lib"
)

var (
	version string
	builtBy string
	builtAt string
	commit  string
)

func main() {
	cmd.Execute(version, builtBy, builtAt, commit)
}
