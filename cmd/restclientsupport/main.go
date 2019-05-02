package main

import (
	"fmt"

	"github.com/harmony-one/harmony/api/service/restclientsupport"
)

func main() {
	s := restclientsupport.New(nil, nil, nil, nil, nil, nil, nil, nil)
	s.StartService()
	fmt.Println("Server started")
	select {}
}
