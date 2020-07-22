package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/genesis"
	"github.com/harmony-one/harmony/numeric"

	"github.com/harmony-one/harmony/p2p"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/shard"

	_ "net/http/pprof"

	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/harmony/internal/cli"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	// TODO: elaborate the usage
	Use:   "harmony",
	Short: "run harmony node",
	Long:  "run harmony node",
	Run:   runHarmonyNode,
}

var configFlag = cli.StringFlag{
	Name:      "config",
	Usage:     "load node config from the config toml file.",
	Shorthand: "c",
	DefValue:  "",
}

func registerRootCmdFlags() error {
	var flags []cli.Flag
	flags = append(flags, configFlag)
	flags = append(flags, generalFlags...)
	flags = append(flags, networkFlags...)
	flags = append(flags, p2pFlags...)
	flags = append(flags, rpcFlags...)
	flags = append(flags, blsFlags...)
	flags = append(flags, consensusFlags...)
	flags = append(flags, txPoolFlags...)
	flags = append(flags, pprofFlags...)
	flags = append(flags, logFlags...)
	flags = append(flags, devnetFlags...)
	flags = append(flags, revertFlags...)
	flags = append(flags, legacyMiscFlags...)

	return cli.RegisterFlags(rootCmd, flags)
}

func runHarmonyNode(cmd *cobra.Command, args []string) {
	prepareRootCmd(cmd)

	cfg, err := getHarmonyConfig(cmd)
	if err != nil {
		fmt.Println(err)
		cmd.Help()
		os.Exit(128)
	}

	setupNodeLog(cfg)
	setupPprof(cfg)
	if err := setupP2P; err != nil {
		fmt.Println(err)
		cmd.Help()
		os.Exit(128)
	}
}

func prepareRootCmd(cmd *cobra.Command) {
	// TODO: can we remove this?
	// HACK Force usage of go implementation rather than the C based one. Do the right way, see the
	// notes one line 66,67 of https://golang.org/src/net/net.go that say can make the decision at
	// build time.
	os.Setenv("GODEBUG", "netdns=go")
	// Don't set higher than num of CPU. It will make go scheduler slower.
	runtime.GOMAXPROCS(runtime.NumCPU())
	// Set up randomization seed.
	rand.Seed(int64(time.Now().Nanosecond()))
}

func getHarmonyConfig(cmd *cobra.Command) (harmonyConfig, error) {
	var (
		config harmonyConfig
		err    error
	)
	if cli.IsFlagChanged(cmd, configFlag) {
		configFile := cli.GetStringFlagValue(cmd, configFlag)
		config, err = loadHarmonyConfig(configFile)
	} else {
		config = getDefaultHmyConfigCopy()
		nt := getNetworkType(cmd)
		config.Network = getDefaultNetworkConfig(nt)
	}
	if err != nil {
		return harmonyConfig{}, err
	}
	// Misc flags shall be applied first since legacy ip / port shall be overwritten
	// by new ip / port flags
	applyLegacyMiscFlags(cmd, &config)
	applyGeneralFlags(cmd, &config)
	applyNetworkFlags(cmd, &config)
	applyP2PFlags(cmd, &config)
	applyRPCFlags(cmd, &config)
	applyBLSFlags(cmd, &config)
	applyConsensusFlags(cmd, &config)
	applyTxPoolFlags(cmd, &config)
	applyPprofFlags(cmd, &config)
	applyLogFlags(cmd, &config)
	applyDevnetFlags(cmd, &config)
	applyRevertFlags(cmd, &config)

	if err := validateHarmonyConfig(config); err != nil {
		return harmonyConfig{}, err
	}

	return config, nil
}

func setupNodeLog(config harmonyConfig) {
	logPath := filepath.Join(config.Log.Folder, config.Log.FileName)
	rotateSize := config.Log.RotateSize
	verbosity := config.Log.Verbosity

	utils.AddLogFile(logPath, rotateSize)
	utils.SetLogVerbosity(log.Lvl(verbosity))
	if config.Log.Context != nil {
		ip := config.Log.Context.IP
		port := config.Log.Context.Port
		utils.SetLogContext(ip, strconv.Itoa(port))
	}
}

func setupPprof(config harmonyConfig) {
	enabled := config.Pprof.Enabled
	addr := config.Pprof.ListenAddr

	if enabled {
		go func() {
			http.ListenAndServe(addr, nil)
		}()
	}
}

func setupP2P(config harmonyConfig) error {
	var err error
	bootNodes := config.Network.BootNodes
	p2p.BootNodes, err = p2p.StringsToAddrs(strings.Join(bootNodes, ","))

	return err
}

func runNodeWithConfig(config harmonyConfig) {

}

func setupNodeConfig(config harmonyConfig) {
	// TODO: seperate use of port rpc / p2p
	nodeconfig.GetDefaultConfig().Port = strconv.Itoa(config.RPC.Port)
	nodeconfig.GetDefaultConfig().IP = config.RPC.IP

	// Set sharding schedule
	// TODO: module interface should have the same signature.
	nodeconfig.SetShardingSchedule(shard.Schedule)

	// TODO: Whether public rpc is defined by user input ip flag. Refactor this.
	nodeconfig.SetPublicRPC(true)

	nodeconfig.SetVersion(getHarmonyVersion())

}

func setShardSchedule(config harmonyConfig) {
	switch config.Network.NetworkType {
	case nodeconfig.Mainnet:
		shard.Schedule = shardingconfig.MainnetSchedule
	case nodeconfig.Testnet:
		shard.Schedule = shardingconfig.TestnetSchedule
	case nodeconfig.Pangaea:
		shard.Schedule = shardingconfig.PangaeaSchedule
	case nodeconfig.Localnet:
		shard.Schedule = shardingconfig.LocalnetSchedule
	case nodeconfig.Partner:
		shard.Schedule = shardingconfig.PartnerSchedule
	case nodeconfig.Stressnet:
		shard.Schedule = shardingconfig.StressNetSchedule
	case nodeconfig.Devnet:
		var dnConfig devnetConfig
		if config.Devnet != nil {
			dnConfig = *config.Devnet
		} else {
			dnConfig = getDefaultDevnetConfigCopy()
		}

		// TODO (leo): use a passing list of accounts here
		devnetConfig, err := shardingconfig.NewInstance(
			uint32(dnConfig.NumShards), dnConfig.ShardSize, dnConfig.HmyNodeSize, numeric.OneDec(), genesis.HarmonyAccounts, genesis.FoundationalNodeAccounts, nil, shardingconfig.VLBPE)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "ERROR invalid devnet sharding config: %s",
				err)
			os.Exit(1)
		}
		shard.Schedule = shardingconfig.NewFixedSchedule(devnetConfig)
	}
}
