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
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/accounts"
	"github.com/harmony-one/harmony/accounts/keystore"
	"github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/shard"
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
	testBLSPubKey       = "b9486167ab9087ab818dc4ce026edb5bf216863364c32e42df2af03c5ced1ad181e7d12f0e6dd5307a73b62247608611"
	testAccountPassword = ""
)

func (s *staker) run(cmd *cobra.Command, args []string) error {
	scryptN := keystore.StandardScryptN
	scryptP := keystore.StandardScryptP
	ks := keystore.NewKeyStore(keystoreDir, scryptN, scryptP)
	account := accounts.Account{Address: dAddr}
	ks.Unlock(account, testAccountPassword)
	gasPrice := big.NewInt(0)
	stakePayloadMaker := func() (staking.Directive, interface{}) {
		p := &bls.PublicKey{}
		p.DeserializeHexStr(testBLSPubKey)
		pub := shard.BlsPublicKey{}
		pub.FromLibBLSPublicKey(p)
		return staking.DirectiveNewValidator, staking.NewValidator{
			Description: staking.Description{
				Name:            "something",
				Identity:        "something else",
				Website:         "some site, harmony.one",
				SecurityContact: "mr.smith",
				Details:         "blah blah details",
			},
			CommissionRates: staking.CommissionRates{
				Rate:          staking.NewDec(100),
				MaxRate:       staking.NewDec(150),
				MaxChangeRate: staking.NewDec(5),
			},
			MinSelfDelegation: big.NewInt(10),
			StakingAddress:    common.Address(dAddr),
			PubKey:            pub,
			Amount:            big.NewInt(100),
		}
		// return message.DirectiveDelegate, message.Delegate{
		// 	common.Address(dAddr),
		// 	common.Address(dAddr),
		// 	big.NewInt(10),
		// }
	}

	stakingTx, err := staking.NewStakingTransaction(2, 100, gasPrice, stakePayloadMaker)
	if err != nil {
		return err
	}
	signed, oops := ks.SignStakingTx(account, stakingTx, localNetChain)
	if oops != nil {
		return oops
	}
	enc, oops1 := rlp.EncodeToBytes(signed)
	if oops1 != nil {
		return oops1
	}
	tx := new(staking.StakingTransaction)
	if err := rlp.DecodeBytes(enc, tx); err != nil {
		return err
	}
	fmt.Printf("In Client side: %+v\n", tx)
	// return nil
	rlp.DecodeBytes(enc, tx)
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
