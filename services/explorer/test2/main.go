package main

import "github.com/harmony-one/harmony/services/explorer"

func main() {
	service := &explorer.Service{}
	service.Init()
	service.Run()
}
