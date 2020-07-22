package main

import (
	"fmt"
)

const (
	versionFormat = "Harmony (C) 2020. %v, version %v-%v (%v %v)"
)

// Version string variables
var (
	version string
	builtBy string
	builtAt string
	commit  string
)

func getHarmonyVersion() string {
	return fmt.Sprintf(versionFormat, "harmony", version, commit, builtBy, builtAt)
}

// TODO add version command here
