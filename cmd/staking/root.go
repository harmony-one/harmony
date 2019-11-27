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

	"github.com/harmony-one/harmony/common/denominations"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/accounts"
	"github.com/harmony-one/harmony/accounts/keystore"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/numeric"
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
	localTest     = true
	localNetChain = big.NewInt(2) // pangaea 3
	devNetChain   = big.NewInt(3) // pangaea 3
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
	rate    = "0.15"

	testAccounts = []string{
		"one103q7qe5t2505lypvltkqtddaef5tzfxwsse4z7",
		"one1uyshu2jgv8w465yc8kkny36thlt2wvel89tcmg",
		"one1r4zyyjqrulf935a479sgqlpa78kz7zlcg2jfen",
		"one1p7ht2d4kl8ve7a8jxw746yfnx4wnfxtp8jqxwe"}
	testBLSPubKeys = []string{
		"678ec9670899bf6af85b877058bea4fc1301a5a3a376987e826e3ca150b80e3eaadffedad0fedfa111576fa76ded980c",
		"a547a9bf6fdde4f4934cde21473748861a3cc0fe8bbb5e57225a29f483b05b72531f002f8187675743d819c955a86100",
		"fc4b9c535ee91f015efff3f32fbb9d32cdd9bfc8a837bb3eee89b8fff653c7af2050a4e147ebe5c7233dc2d5df06ee0a",
		"ca86e551ee42adaaa6477322d7db869d3e203c00d7b86c82ebee629ad79cb6d57b8f3db28336778ec2180e56a8e07296"}
)

func (s *staker) run(cmd *cobra.Command, args []string) error {
	scryptN := keystore.StandardScryptN
	scryptP := keystore.StandardScryptP
	ks := keystore.NewKeyStore(keystoreDir, scryptN, scryptP)
	dAddr, _ := common2.Bech32ToAddress(testAccounts[index])
	dAddr2, _ := common2.Bech32ToAddress(testAccounts[0])
	account := accounts.Account{Address: dAddr}
	ks.Unlock(account, testAccountPassword)
	gasPrice := big.NewInt(1)
	stakePayloadMaker := func() (staking.Directive, interface{}) {
		p := &bls.PublicKey{}
		p.DeserializeHexStr(testBLSPubKeys[index])
		pub := shard.BlsPublicKey{}
		pub.FromLibBLSPublicKey(p)

		ra, _ := numeric.NewDecFromStr("0.2")
		maxRate, _ := numeric.NewDecFromStr("1")
		maxChangeRate, _ := numeric.NewDecFromStr("0.05")
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
				MinSelfDelegation:  big.NewInt(denominations.One),
				MaxTotalDelegation: big.NewInt(0).Mul(big.NewInt(denominations.One), big.NewInt(1000)),
				ValidatorAddress:   common.Address(dAddr),
				SlotPubKeys:        []shard.BlsPublicKey{pub},
				Amount:             big.NewInt(denominations.One),
			}
		} else if cmdType == "edit" {
			newRate, _ := numeric.NewDecFromStr(rate)
			edit := staking.EditValidator{
				Description: &staking.Description{
					Name: name,
				},
				CommissionRate:   &newRate,
				ValidatorAddress: common.Address(dAddr),
			}
			return staking.DirectiveEditValidator, edit
		} else if cmdType == "delegate" {
			return staking.DirectiveDelegate, staking.Delegate{
				dAddr,
				dAddr2,
				big.NewInt(1000),
			}
		} else if cmdType == "undelegate" {
			return staking.DirectiveUndelegate, staking.Undelegate{
				dAddr,
				dAddr2,
				big.NewInt(1000),
			}
		}
		return staking.DirectiveCollectRewards, staking.CollectRewards{
			dAddr,
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

	}

	stakingTx, err := staking.NewStakingTransaction(uint64(nonce), 600000, gasPrice, stakePayloadMaker)
	if err != nil {
		return err
	}
	chainID := localNetChain
	if !localTest {
		chainID = devNetChain
	}
	signed, oops := ks.SignStakingTx(account, stakingTx, chainID)
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

	local := "http://localhost:9500"
	dev := "https://api.s0.pga.hmny.io/" //"http://34.221.51.210:9500" // "https://api.s0.pga.hmny.io/"
	net := local
	if !localTest {
		net = dev
	}
	// Check Double check if have enough balance for gas fees
	balanceR, _ := baseRequest("hmy_getBalance", net, []interface{}{testAccounts[index], "latest"})
	m := map[string]interface{}{}
	json.Unmarshal(balanceR, &m)
	balance, _ := m["result"].(string)
	bln, _ := big.NewInt(0).SetString(balance[2:], 16)

	if bln.Cmp(big.NewInt(0)) == 0 {
		fmt.Println("Balance for ", testAccounts[index], "is zero, tx will be rejected b/c not enough for gas fee, exiting")
		os.Exit(-1)
	}

	fmt.Println("balance", convertBalanceIntoReadableFormat(bln))

	result, reqOops := baseRequest(stakingRPC, net, param)
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
	rootCmd.PersistentFlags().BoolVarP(&localTest, "local", "l", true, "local test")

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

func convertBalanceIntoReadableFormat(balance *big.Int) string {
	balance = balance.Div(balance, big.NewInt(denominations.Nano))
	strBalance := fmt.Sprintf("%d", balance.Uint64())

	bytes := []byte(strBalance)
	hasDecimal := false
	for i := 0; i < 11; i++ {
		if len(bytes)-1-i < 0 {
			bytes = append([]byte{'0'}, bytes...)
		}
		if bytes[len(bytes)-1-i] != '0' && i < 9 {
			hasDecimal = true
		}
		if i == 9 {
			newBytes := append([]byte{'.'}, bytes[len(bytes)-i:]...)
			bytes = append(bytes[:len(bytes)-i], newBytes...)
		}
	}
	zerosToRemove := 0
	for i := 0; i < len(bytes); i++ {
		if hasDecimal {
			if bytes[len(bytes)-1-i] == '0' {
				bytes = bytes[:len(bytes)-1-i]
				i--
			} else {
				break
			}
		} else {
			if zerosToRemove < 5 {
				bytes = bytes[:len(bytes)-1-i]
				i--
				zerosToRemove++
			} else {
				break
			}
		}
	}
	return string(bytes)
}
