package main

import "github.com/harmony-one/harmony/services/explorer"

func main() {
	service := &explorer.Service{
		IP:   "127.0.0.1",
		Port: "9000",
	}
	service.Init(false)
	service.Run()
}
