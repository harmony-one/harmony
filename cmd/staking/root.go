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

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/accounts"
	"github.com/harmony-one/harmony/accounts/keystore"
	common2 "github.com/harmony-one/harmony/internal/common"
	numeric "github.com/harmony-one/harmony/numeric"
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
)

const (
	// Harmony protocol assume beacon chain shard is only place to send
	// staking, later need to consider logic when beacon chain shard rotates
	stakingShard        = 0
	testAccountPassword = ""
)

// command line options var
var (
	nonce   = 0
	cmdType = "create"
	name    = "NewName"
	index   = 0
	minDele = 777
	rate    = "0.0"

	testAccounts = []string{
		"one1pdv9lrdwl0rg5vglh4xtyrv3wjk3wsqket7zxy",
		"one12fuf7x9rgtdgqg7vgq0962c556m3p7afsxgvll"}
	testBLSPubKeys = []string{
		"65f55eb3052f9e9f632b2923be594ba77c55543f5c58ee1454b9cfd658d25e06373b0f7d42a19c84768139ea294f6204",
		"02c8ff0b88f313717bc3a627d2f8bb172ba3ad3bb9ba3ecb8eed4b7c878653d3d4faf769876c528b73f343967f74a917"}
)

func (s *staker) run(cmd *cobra.Command, args []string) error {
	scryptN := keystore.StandardScryptN
	scryptP := keystore.StandardScryptP
	ks := keystore.NewKeyStore(keystoreDir, scryptN, scryptP)
	dAddr, _ := common2.Bech32ToAddress(testAccounts[index])
	account := accounts.Account{Address: dAddr}
	ks.Unlock(account, testAccountPassword)
	gasPrice := big.NewInt(1)
	stakePayloadMaker := func() (staking.Directive, interface{}) {
		p := &bls.PublicKey{}
		p.DeserializeHexStr(testBLSPubKeys[index])
		pub := shard.BlsPublicKey{}
		pub.FromLibBLSPublicKey(p)

		ra, _ := numeric.NewDecFromStr("27.27")
		maxRate, _ := numeric.NewDecFromStr("150.99")
		maxChangeRate, _ := numeric.NewDecFromStr("0.5")
		if cmdType == "create" {
			return staking.DirectiveCreateValidator, staking.CreateValidator{
				Description: &staking.Description{
					Name:            "SuperHero",
					Identity:        "YouWouldNotKnow",
					Website:         "Secret Website",
					SecurityContact: "Mr.DoubleZeroSeven",
					Details:         "blah blah blah",
				},
				CommissionRates: staking.CommissionRates{
					Rate:          ra,
					MaxRate:       maxRate,
					MaxChangeRate: maxChangeRate,
				},
				MinSelfDelegation:  big.NewInt(10),
				MaxTotalDelegation: big.NewInt(3000),
				ValidatorAddress:   common.Address(dAddr),
				SlotPubKeys:        []shard.BlsPublicKey{pub},
				Amount:             big.NewInt(100),
			}
		}
		/*
			ValidatorAddress   common.Address      `json:"validator_address" yaml:"validator_address"`
				Description        *Description        `json:"description" yaml:"description"`
				CommissionRate     *numeric.Dec        `json:"commission_rate" yaml:"commission_rate"`
				MinSelfDelegation  *big.Int            `json:"min_self_delegation" yaml:"min_self_delegation"`
				MaxTotalDelegation *big.Int            `json:"max_total_delegation" yaml:"max_total_delegation"`
				SlotKeyToRemove    *shard.BlsPublicKey `json:"slot_key_to_remove" yaml:"slot_key_to_remove"`
				SlotKeyToAdd       *shard.BlsPublicKey `json:"slot_key_to_add" yaml:"slot_key_to_add"`
			}
		*/

		newRate, _ := numeric.NewDecFromStr(rate)
		return staking.DirectiveEditValidator, staking.EditValidator{
			Description: &staking.Description{
				Name: name,
			},
			MinSelfDelegation: big.NewInt(int64(minDele)),
			CommissionRate:    &newRate,
			ValidatorAddress:  common.Address(dAddr),
		}
	}

	stakingTx, err := staking.NewStakingTransaction(uint64(nonce), 600000, gasPrice, stakePayloadMaker)
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

	payload, err := tx.RLPEncodeStakeMsg()

	restored, errRestor := staking.RLPDecodeStakeMsg(
		payload, staking.DirectiveCreateValidator,
	)

	fmt.Printf("In Client side: %+v\n", restored)
	fmt.Println(errRestor)

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
	rootCmd.PersistentFlags().IntVarP(&index, "index", "i", 0, "account index:0|1")
	rootCmd.PersistentFlags().IntVarP(&nonce, "nonce", "n", 0, "nonce of address")
	rootCmd.PersistentFlags().StringVarP(&cmdType, "type", "t", "create", "type of commands: create|edit")
	rootCmd.PersistentFlags().StringVarP(&name, "name", "m", "ANewName", "Name of Validator")
	rootCmd.PersistentFlags().IntVarP(&minDele, "minDele", "d", 666, "MinSelfDelegation Fee")
	rootCmd.PersistentFlags().StringVarP(&rate, "rate", "r", "22.22", "Commision Rate")

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
