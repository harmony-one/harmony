package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"os"
	"path"
	"strconv"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/accounts"
	"github.com/harmony-one/harmony/accounts/keystore"
	"github.com/harmony-one/harmony/internal/common"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/spf13/cobra"
)

const (
	keystoreDir = ".hmy/keystore"
	stakingRPC  = "hmy_sendRawStakingTransaction"
)

type staker struct{}

var (
	queryID       = 0
	s             = &staker{}
	localNetChain = big.NewInt(2)
	dAddr         = common.ParseAddr(testAccount)
)

const (
	// Harmony protocol assume beacon chain shard is only place to send
	// staking, later need to consider logic when beacon chain shard rotates
	stakingShard        = 0
	testAccount         = "one1a0x3d6xpmr6f8wsyaxd9v36pytvp48zckswvv9"
	testAccountPassword = ""
)

func (s *staker) preRunInit(cmd *cobra.Command, args []string) error {
	// Just in case need to do some kind of setup that needs to propagate downward
	return nil
}

func baseRequest(method, node string, params interface{}) ([]byte, error) {
	requestBody, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      strconv.Itoa(queryID),
		"method":  method,
		"params":  params,
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
	queryID++
	fmt.Printf("URL: %s, Response Body: %s\n\n", node, string(body))
	return body, err
}

func (s *staker) run(cmd *cobra.Command, args []string) error {
	scryptN := keystore.StandardScryptN
	scryptP := keystore.StandardScryptP
	ks := keystore.NewKeyStore(keystoreDir, scryptN, scryptP)
	account := accounts.Account{Address: dAddr}
	ks.Unlock(account, testAccountPassword)
	gasPrice := big.NewInt(0)
	sMessage, oops := staking.NewMessage(staking.Delegate, common.Address(dAddr))
	if oops != nil {
		return oops
	}
	stakingTx := staking.NewTransaction(2, 100, gasPrice, sMessage)
	signed, oops := ks.SignStakingTx(account, stakingTx, localNetChain)
	enc, oops1 := rlp.EncodeToBytes(signed)
	if oops1 != nil {
		return oops1
	}
	hexSignature := hexutil.Encode(enc)
	param := []interface{}{hexSignature}
	result, reqOops := baseRequest(stakingRPC, "http://localhost:9500", param)
	fmt.Println(string(result))
	return reqOops
}

func versionS() string {
	return fmt.Sprintf(
		"Harmony (C) 2019. %v, version %v-%v (%v %v)",
		path.Base(os.Args[0]), version, commit, builtBy, builtAt,
	)
}

func init() {

	rootCmd.AddCommand(&cobra.Command{
		Use:               "staking-iterate",
		Short:             "run through staking process",
		PersistentPreRunE: s.preRunInit,
		RunE:              s.run,
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Show version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(os.Stderr, versionS()+"\n")
			os.Exit(0)
		},
	})
}

var (
	rootCmd = &cobra.Command{
		Use:          "staking-standalone",
		SilenceUsage: true,
		Long:         "testing staking quickly",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
)
