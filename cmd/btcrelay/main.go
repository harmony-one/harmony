// Copyright (c) 2014-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"

	"github.com/btcsuite/btcd/rpcclient"
)

var (
	version string
	builtBy string
	builtAt string
	commit  string
)

func printVersion(me string) {
	fmt.Fprintf(os.Stderr, "Harmony (C) 2020. %v, version %v-%v (%v %v)\n", path.Base(me), version, commit, builtBy, builtAt)
	os.Exit(0)
}

func main() {
	block := flag.Int64("block", -1, "bitcoin block number (-1: latest block)")
	ip := flag.String("ip", "127.0.0.1", "IP of the bitcoin node")
	port := flag.String("port", "8332", "RPC port of the bitcoin node")
	user := flag.String("user", "", "user name to access the bitcoin rpc")
	passwd := flag.String("pass", "", "password to access the bitcoin rpc")
	versionFlag := flag.Bool("version", false, "Output version info")

	flag.Parse()

	if *versionFlag {
		printVersion(os.Args[0])
	}

	// Connect to local bitcoin core RPC server using HTTP POST mode.
	connCfg := &rpcclient.ConnConfig{
		Host:         fmt.Sprintf("%s:%s", *ip, *port),
		User:         *user,
		Pass:         *passwd,
		HTTPPostMode: true, // Bitcoin core only supports HTTP POST mode
		DisableTLS:   true, // Bitcoin core does not provide TLS by default
	}
	// Notice the notification parameter is nil since notifications are
	// not supported in HTTP POST mode.
	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Shutdown()

	// Get the current block count.
	blockCount, err := client.GetBlockCount()
	if err != nil {
		log.Fatal(err)
	}

	blockNum := *block
	if *block == -1 {
		blockNum = blockCount
	}
	log.Printf("Get block: %d", blockNum)

	blockHash, err := client.GetBlockHash(blockNum)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Block hash: %s", blockHash)
	blockHeader, err := client.GetBlockHeader(blockHash)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Block: %v", blockHeader)
}
