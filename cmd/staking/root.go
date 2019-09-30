package main

import (
	"fmt"
	"math/big"
	"os"
	"path"

	"github.com/harmony-one/harmony/accounts"
	"github.com/harmony-one/harmony/accounts/keystore"
	"github.com/harmony-one/harmony/internal/common"
	types2 "github.com/harmony-one/harmony/staking/types"
	"github.com/spf13/cobra"
)

const (
	keystoreDir = ".hmy/keystore"
)

type staker struct {
}

var (
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

type dm struct{}

func (d dm) Type() string           { return "foo-bar" }
func (d dm) Signer() common.Address { return common.Address(dAddr) }

func (s *staker) run(cmd *cobra.Command, args []string) error {
	scryptN := keystore.StandardScryptN
	scryptP := keystore.StandardScryptP
	ks := keystore.NewKeyStore(keystoreDir, scryptN, scryptP)
	account := accounts.Account{Address: dAddr}
	ks.Unlock(account, testAccountPassword)
	stakingTx := types2.NewTransaction(2, 100, big.NewInt(0), dm{})
	signed, oops := ks.SignStakingTx(account, stakingTx, localNetChain)
	fmt.Println(signed)
	return oops
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
