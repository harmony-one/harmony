package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/api/service/syncing"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/blsgen"
	"github.com/harmony-one/harmony/internal/common"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	viperconfig "github.com/harmony-one/harmony/internal/configs/viper"
	"github.com/harmony-one/harmony/internal/genesis"
	"github.com/harmony-one/harmony/internal/shardchain"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/node"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/webhooks"
	"github.com/pkg/errors"
)

// Version string variables
var (
	version string
	builtBy string
	builtAt string
	commit  string
)

// Host
var (
	myHost          p2p.Host
	initialAccounts = []*genesis.DeployAccount{}
)

func printVersion() {
	fmt.Fprintln(os.Stderr, nodeconfig.GetVersion())
	os.Exit(0)
}

var (
	ip          = flag.String("ip", "127.0.0.1", "ip of the node")
	port        = flag.String("port", "9000", "port of the node.")
	logFolder   = flag.String("log_folder", "latest", "the folder collecting the logs of this execution")
	logMaxSize  = flag.Int("log_max_size", 100, "the max size in megabytes of the log file before it gets rotated")
	freshDB     = flag.Bool("fresh_db", false, "true means the existing disk based db will be removed")
	pprof       = flag.String("pprof", "", "what address and port the pprof profiling server should listen on")
	versionFlag = flag.Bool("version", false, "Output version info")
	dnsZone     = flag.String("dns_zone", "", "if given and not empty, use peers from the zone (default: use libp2p peer discovery instead)")
	dnsFlag     = flag.Bool("dns", true, "[deprecated] equivalent to -dns_zone t.hmny.io")
	dnsPort     = flag.String("dns_port", "9000", "port of dns node")
	//Leader needs to have a minimal number of peers to start consensus
	minPeers = flag.Int("min_peers", 32, "Minimal number of Peers in shard")
	// Key file to store the private key
	keyFile = flag.String("key", "./.hmykey", "the p2p key file of the harmony node")
	// isArchival indicates this node is an archival node that will save and archive current blockchain
	isArchival = flag.Bool("is_archival", false, "false will enable cached state pruning")
	// delayCommit is the commit-delay timer, used by Harmony nodes
	delayCommit = flag.String("delay_commit", "0ms", "how long to delay sending commit messages in consensus, ex: 500ms, 1s")
	// nodeType indicates the type of the node: validator, explorer
	nodeType = flag.String("node_type", "validator", "node type: validator, explorer")
	// networkType indicates the type of the network
	networkType = flag.String("network_type", "mainnet", "type of the network: mainnet, testnet, pangaea, partner, stressnet, devnet, localnet")
	// blockPeriod indicates the how long the leader waits to propose a new block.
	blockPeriod = flag.Int("block_period", 8, "how long in second the leader waits to propose a new block.")
	// staking indicates whether the node is operating in staking mode.
	stakingFlag = flag.Bool("staking", false, "whether the node should operate in staking mode")
	// shardID indicates the shard ID of this node
	shardID            = flag.Int("shard_id", -1, "the shard ID of this node")
	cmkEncryptedBLSKey = flag.String("aws_blskey", "", "The aws CMK encrypted bls private key file.")
	blsKeyFile         = flag.String("blskey_file", "", "The encrypted file of bls serialized private key by passphrase.")
	blsFolder          = flag.String("blsfolder", ".hmy/blskeys", "The folder that stores the bls keys and corresponding passphrases; e.g. <blskey>.key and <blskey>.pass; all bls keys mapped to same shard")
	blsPass            = flag.String("blspass", "", "The file containing passphrase to decrypt the encrypted bls file.")
	blsPassphrase      string
	maxBLSKeysPerNode  = flag.Int("max_bls_keys_per_node", 4, "maximum number of bls keys allowed per node (default 4)")
	// Sharding configuration parameters for devnet
	devnetNumShards   = flag.Uint("dn_num_shards", 2, "number of shards for -network_type=devnet (default: 2)")
	devnetShardSize   = flag.Int("dn_shard_size", 10, "number of nodes per shard for -network_type=devnet (default 10)")
	devnetHarmonySize = flag.Int("dn_hmy_size", -1, "number of Harmony-operated nodes per shard for -network_type=devnet; negative (default) means equal to -dn_shard_size")
	// logging verbosity
	verbosity = flag.Int("verbosity", 5, "Logging verbosity: 0=silent, 1=error, 2=warn, 3=info, 4=debug, 5=detail (default: 5)")
	// dbDir is the database directory.
	dbDir     = flag.String("db_dir", "", "blockchain database directory")
	publicRPC = flag.Bool("public_rpc", false, "Enable Public RPC Access (default: false)")
	// Bad block revert
	doRevertBefore = flag.Int("do_revert_before", 0, "If the current block is less than do_revert_before, revert all blocks until (including) revert_to block")
	revertTo       = flag.Int("revert_to", 0, "The revert will rollback all blocks until and including block number revert_to")
	revertBeacon   = flag.Bool("revert_beacon", false, "Whether to revert beacon chain or the chain this node is assigned to")
	// Blacklist of addresses
	blacklistPath      = flag.String("blacklist", "./.hmy/blacklist.txt", "Path to newline delimited file of blacklisted wallet addresses")
	broadcastInvalidTx = flag.Bool("broadcast_invalid_tx", false, "Broadcast invalid transactions to sync pool state (default: false)")
	webHookYamlPath    = flag.String(
		"webhook_yaml", "", "path for yaml config reporting double signing",
	)
	// aws credentials
	awsSettingString = ""
)

func initSetup() {

	// Setup pprof
	if addr := *pprof; addr != "" {
		go func() { http.ListenAndServe(addr, nil) }()
	}

	// maybe request passphrase for bls key.
	if *cmkEncryptedBLSKey == "" {
		passphraseForBLS()
	} else {
		// Get aws credentials from stdin prompt
		awsSettingString, _ = blsgen.Readln(1 * time.Second)
	}

	// Configure log parameters
	utils.SetLogContext(*port, *ip)
	utils.SetLogVerbosity(log.Lvl(*verbosity))
	utils.AddLogFile(fmt.Sprintf("%v/validator-%v-%v.log", *logFolder, *ip, *port), *logMaxSize)

	// Don't set higher than num of CPU. It will make go scheduler slower.
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Set port and ip to global config.
	nodeconfig.GetDefaultConfig().Port = *port
	nodeconfig.GetDefaultConfig().IP = *ip

	// Set sharding schedule
	nodeconfig.SetShardingSchedule(shard.Schedule)

	// Set up randomization seed.
	rand.Seed(int64(time.Now().Nanosecond()))

	if len(p2p.BootNodes) == 0 {
		bootNodeAddrs, err := p2p.StringsToAddrs(p2p.DefaultBootNodeAddrStrings)
		if err != nil {
			utils.FatalErrMsg(err, "cannot parse default bootnode list %#v",
				p2p.DefaultBootNodeAddrStrings)
		}
		p2p.BootNodes = bootNodeAddrs
	}
}

func passphraseForBLS() {
	// If FN node running, they should either specify blsPrivateKey or the file with passphrase
	// However, explorer or non-validator nodes need no blskey
	if *nodeType != "validator" {
		return
	}

	if *blsKeyFile == "" && *blsFolder == "" {
		fmt.Println("blskey_file or blsfolder option must be provided")
		os.Exit(101)
	}
	if *blsPass == "" {
		fmt.Println("Internal nodes need to have blspass to decrypt blskey")
		os.Exit(101)
	}
	fmt.Println(*blsPass)
	passphrase, err := utils.GetPassphraseFromSource(*blsPass)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "ERROR when reading passphrase file: %v\n", err)
		os.Exit(100)
	}
	blsPassphrase = passphrase
}

func findAccountsByPubKeys(config shardingconfig.Instance, pubKeys multibls.PublicKeys) {
	for _, key := range pubKeys {
		keyStr := key.Bytes.Hex()
		_, account := config.FindAccount(keyStr)
		if account != nil {
			initialAccounts = append(initialAccounts, account)
		}
	}
}

func setupLegacyNodeAccount() error {
	genesisShardingConfig := shard.Schedule.InstanceForEpoch(big.NewInt(core.GenesisEpoch))
	multiBLSPubKey := setupConsensusKey(nodeconfig.GetDefaultConfig()).GetPublicKeys()

	reshardingEpoch := genesisShardingConfig.ReshardingEpoch()
	if len(reshardingEpoch) > 0 {
		for _, epoch := range reshardingEpoch {
			config := shard.Schedule.InstanceForEpoch(epoch)
			findAccountsByPubKeys(config, multiBLSPubKey)
			if len(initialAccounts) != 0 {
				break
			}
		}
	} else {
		findAccountsByPubKeys(genesisShardingConfig, multiBLSPubKey)
	}

	if len(initialAccounts) == 0 {
		fmt.Fprintf(
			os.Stderr,
			"ERROR cannot find your BLS key in the genesis/FN tables: %s\n",
			multiBLSPubKey.SerializeToHexStr(),
		)
		os.Exit(100)
	}

	for _, account := range initialAccounts {
		fmt.Printf("My Genesis Account: %v\n", *account)
	}
	return nil
}

func setupStakingNodeAccount() error {
	pubKey := setupConsensusKey(nodeconfig.GetDefaultConfig()).GetPublicKeys()
	shardID, err := nodeconfig.GetDefaultConfig().ShardIDFromConsensusKey()
	if err != nil {
		return errors.Wrap(err, "cannot determine shard to join")
	}
	if err := nodeconfig.GetDefaultConfig().ValidateConsensusKeysForSameShard(
		pubKey, shardID,
	); err != nil {
		return err
	}
	for _, blsKey := range pubKey {
		initialAccount := &genesis.DeployAccount{}
		initialAccount.ShardID = shardID
		initialAccount.BLSPublicKey = blsKey.Bytes.Hex()
		initialAccount.Address = ""
		initialAccounts = append(initialAccounts, initialAccount)
	}
	return nil
}

func readMultiBLSKeys(consensusMultiBLSPriKey *multibls.PrivateKeys) error {
	keyPasses := map[string]string{}
	blsKeyFiles := []os.FileInfo{}
	awsEncryptedBLSKeyFiles := []os.FileInfo{}

	if err := filepath.Walk(*blsFolder, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		fullName := info.Name()
		ext := filepath.Ext(fullName)
		if ext == ".key" {
			blsKeyFiles = append(blsKeyFiles, info)
		} else if ext == ".pass" {
			passFileName := "file:" + path
			passphrase, err := utils.GetPassphraseFromSource(passFileName)
			if err != nil {
				return err
			}
			name := fullName[:len(fullName)-len(ext)]
			keyPasses[name] = passphrase
		} else if ext == ".bls" {
			awsEncryptedBLSKeyFiles = append(awsEncryptedBLSKeyFiles, info)
		} else {
			return errors.Errorf(
				"[Multi-BLS] found file: %s that does not have .bls, .key or .pass file extension",
				path,
			)
		}
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr,
			"[Multi-BLS] ERROR when reading blskey file under %s: %v\n",
			*blsFolder,
			err,
		)
		os.Exit(100)
	}

	var keyFiles []os.FileInfo
	legacyBLSFile := true

	if len(awsEncryptedBLSKeyFiles) > 0 {
		keyFiles = awsEncryptedBLSKeyFiles
		legacyBLSFile = false
	} else {
		keyFiles = blsKeyFiles
	}

	if len(keyFiles) > *maxBLSKeysPerNode {
		fmt.Fprintf(os.Stderr,
			"[Multi-BLS] maximum number of bls keys per node is %d, found: %d\n",
			*maxBLSKeysPerNode,
			len(keyFiles),
		)
		os.Exit(100)
	}

	for _, blsKeyFile := range keyFiles {
		var consensusPriKey *bls.SecretKey
		var err error
		blsKeyFilePath := path.Join(*blsFolder, blsKeyFile.Name())
		if legacyBLSFile {
			fullName := blsKeyFile.Name()
			ext := filepath.Ext(fullName)
			name := fullName[:len(fullName)-len(ext)]
			if val, ok := keyPasses[name]; ok {
				blsPassphrase = val
			}
			consensusPriKey, err = blsgen.LoadBLSKeyWithPassPhrase(blsKeyFilePath, blsPassphrase)
		} else {
			consensusPriKey, err = blsgen.LoadAwsCMKEncryptedBLSKey(blsKeyFilePath, awsSettingString)
		}
		if err != nil {
			return err
		}

		*consensusMultiBLSPriKey = append(*consensusMultiBLSPriKey, multibls.GetPrivateKeys(consensusPriKey)...)
	}

	return nil
}

func setupConsensusKey(nodeConfig *nodeconfig.ConfigType) multibls.PrivateKeys {
	consensusMultiPriKey := multibls.PrivateKeys{}

	if *blsKeyFile != "" {
		consensusPriKey, err := blsgen.LoadBLSKeyWithPassPhrase(*blsKeyFile, blsPassphrase)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR when loading bls key, err :%v\n", err)
			os.Exit(100)
		}
		consensusMultiPriKey = append(consensusMultiPriKey, multibls.GetPrivateKeys(consensusPriKey)...)
	} else if *cmkEncryptedBLSKey != "" {
		consensusPriKey, err := blsgen.LoadAwsCMKEncryptedBLSKey(*cmkEncryptedBLSKey, awsSettingString)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR when loading aws CMK encrypted bls key, err :%v\n", err)
			os.Exit(100)
		}

		consensusMultiPriKey = append(consensusMultiPriKey, multibls.GetPrivateKeys(consensusPriKey)...)
	} else {
		err := readMultiBLSKeys(&consensusMultiPriKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[Multi-BLS] ERROR when loading bls keys, err :%v\n", err)
			os.Exit(100)
		}
	}

	// Consensus keys are the BLS12-381 keys used to sign consensus messages
	nodeConfig.ConsensusPriKey = consensusMultiPriKey

	return consensusMultiPriKey
}

func createGlobalConfig() (*nodeconfig.ConfigType, error) {
	var err error

	if len(initialAccounts) == 0 {
		initialAccounts = append(initialAccounts, &genesis.DeployAccount{ShardID: uint32(*shardID)})
	}
	nodeConfig := nodeconfig.GetShardConfig(initialAccounts[0].ShardID)
	if *nodeType == "validator" {
		// Set up consensus keys.
		setupConsensusKey(nodeConfig)
	} else {
		// set dummy bls key for consensus object
		nodeConfig.ConsensusPriKey = multibls.GetPrivateKeys(&bls.SecretKey{})
	}

	// Set network type
	netType := nodeconfig.NetworkType(*networkType)
	nodeconfig.SetNetworkType(netType) // sets for both global and shard configs
	nodeConfig.SetArchival(*isArchival)

	// P2P private key is used for secure message transfer between p2p nodes.
	nodeConfig.P2PPriKey, _, err = utils.LoadKeyFromFile(*keyFile)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot load or create P2P key at %#v",
			*keyFile)
	}

	selfPeer := p2p.Peer{
		IP:              *ip,
		Port:            *port,
		ConsensusPubKey: nodeConfig.ConsensusPriKey[0].Pub.Object,
	}

	myHost, err = p2p.NewHost(&selfPeer, nodeConfig.P2PPriKey)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create P2P network host")
	}

	nodeConfig.DBDir = *dbDir

	if p := *webHookYamlPath; p != "" {
		config, err := webhooks.NewWebHooksFromPath(p)
		if err != nil {
			fmt.Fprintf(
				os.Stderr, "yaml path is bad: %s", p,
			)
			os.Exit(1)
		}
		nodeConfig.WebHooks.Hooks = config
	}

	return nodeConfig, nil
}

func setupConsensusAndNode(nodeConfig *nodeconfig.ConfigType) *node.Node {
	// Consensus object.
	// TODO: consensus object shouldn't start here
	decider := quorum.NewDecider(quorum.SuperMajorityVote, uint32(*shardID))

	currentConsensus, err := consensus.New(
		myHost, nodeConfig.ShardID, p2p.Peer{}, nodeConfig.ConsensusPriKey, decider,
	)
	currentConsensus.Decider.SetMyPublicKeyProvider(func() (multibls.PublicKeys, error) {
		return currentConsensus.GetPublicKeys(), nil
	})

	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error :%v \n", err)
		os.Exit(1)
	}
	commitDelay, err := time.ParseDuration(*delayCommit)
	if err != nil || commitDelay < 0 {
		_, _ = fmt.Fprintf(os.Stderr, "ERROR invalid commit delay %#v", *delayCommit)
		os.Exit(1)
	}
	currentConsensus.SetCommitDelay(commitDelay)
	currentConsensus.MinPeers = *minPeers

	blacklist, err := setupBlacklist()
	if err != nil {
		utils.Logger().Warn().Msgf("Blacklist setup error: %s", err.Error())
	}

	// Current node.
	chainDBFactory := &shardchain.LDBFactory{RootDir: nodeConfig.DBDir}

	currentNode := node.New(myHost, currentConsensus, chainDBFactory, blacklist, *isArchival)
	currentNode.BroadcastInvalidTx = *broadcastInvalidTx

	switch {
	case *networkType == nodeconfig.Localnet:
		epochConfig := shard.Schedule.InstanceForEpoch(ethCommon.Big0)
		selfPort, err := strconv.ParseUint(*port, 10, 16)
		if err != nil {
			utils.Logger().Fatal().
				Err(err).
				Str("self_port_string", *port).
				Msg("cannot convert self port string into port number")
		}
		currentNode.SyncingPeerProvider = node.NewLocalSyncingPeerProvider(
			6000, uint16(selfPort), epochConfig.NumShards(), uint32(epochConfig.NumNodesPerShard()))
	case *dnsZone != "":
		currentNode.SyncingPeerProvider = node.NewDNSSyncingPeerProvider(*dnsZone, syncing.GetSyncingPort(*dnsPort))
	case *dnsFlag:
		currentNode.SyncingPeerProvider = node.NewDNSSyncingPeerProvider("t.hmny.io", syncing.GetSyncingPort(*dnsPort))
	default:
		currentNode.SyncingPeerProvider = node.NewLegacySyncingPeerProvider(currentNode)

	}

	// TODO: refactor the creation of blockchain out of node.New()
	currentConsensus.ChainReader = currentNode.Blockchain()
	currentNode.NodeConfig.DNSZone = *dnsZone

	currentNode.NodeConfig.SetBeaconGroupID(
		nodeconfig.NewGroupIDByShardID(shard.BeaconChainShardID),
	)

	nodeconfig.GetDefaultConfig().DBDir = nodeConfig.DBDir
	switch *nodeType {
	case "explorer":
		nodeconfig.SetDefaultRole(nodeconfig.ExplorerNode)
		currentNode.NodeConfig.SetRole(nodeconfig.ExplorerNode)
	case "validator":
		nodeconfig.SetDefaultRole(nodeconfig.Validator)
		currentNode.NodeConfig.SetRole(nodeconfig.Validator)
	}
	currentNode.NodeConfig.SetShardGroupID(nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(nodeConfig.ShardID)))
	currentNode.NodeConfig.SetClientGroupID(nodeconfig.NewClientGroupIDByShardID(shard.BeaconChainShardID))
	currentNode.NodeConfig.ConsensusPriKey = nodeConfig.ConsensusPriKey

	// This needs to be executed after consensus setup
	if err := currentNode.InitConsensusWithValidators(); err != nil {
		utils.Logger().Warn().
			Int("shardID", *shardID).
			Err(err).
			Msg("InitConsensusWithMembers failed")
	}

	// Set the consensus ID to be the current block number
	viewID := currentNode.Blockchain().CurrentBlock().Header().ViewID().Uint64()
	currentConsensus.SetViewID(viewID + 1)
	utils.Logger().Info().
		Uint64("viewID", viewID).
		Msg("Init Blockchain")

	// Assign closure functions to the consensus object
	currentConsensus.BlockVerifier = currentNode.VerifyNewBlock
	currentConsensus.OnConsensusDone = currentNode.PostConsensusProcessing
	// update consensus information based on the blockchain
	currentConsensus.SetMode(currentConsensus.UpdateConsensusInformation())
	// Setup block period and block due time.
	currentConsensus.BlockPeriod = time.Duration(*blockPeriod) * time.Second
	currentConsensus.NextBlockDue = time.Now()
	return currentNode
}

func setupBlacklist() (map[ethCommon.Address]struct{}, error) {
	utils.Logger().Debug().Msgf("Using blacklist file at `%s`", *blacklistPath)
	dat, err := ioutil.ReadFile(*blacklistPath)
	if err != nil {
		return nil, err
	}
	addrMap := make(map[ethCommon.Address]struct{})
	for _, line := range strings.Split(string(dat), "\n") {
		if len(line) != 0 { // blacklist file may have trailing empty string line
			b32 := strings.TrimSpace(strings.Split(string(line), "#")[0])
			addr, err := common.Bech32ToAddress(b32)
			if err != nil {
				return nil, err
			}
			addrMap[addr] = struct{}{}
		}
	}
	return addrMap, nil
}

func setupViperConfig() {
	// read from environment
	envViper := viperconfig.CreateEnvViper()
	//read from config file
	configFileViper := viperconfig.CreateConfFileViper("./.hmy", "nodeconfig", "json")
	viperconfig.ResetConfString(ip, envViper, configFileViper, "", "ip")
	viperconfig.ResetConfString(port, envViper, configFileViper, "", "port")
	viperconfig.ResetConfString(logFolder, envViper, configFileViper, "", "log_folder")
	viperconfig.ResetConfInt(logMaxSize, envViper, configFileViper, "", "log_max_size")
	viperconfig.ResetConfBool(freshDB, envViper, configFileViper, "", "fresh_db")
	viperconfig.ResetConfString(pprof, envViper, configFileViper, "", "pprof")
	viperconfig.ResetConfBool(versionFlag, envViper, configFileViper, "", "version")
	viperconfig.ResetConfString(dnsZone, envViper, configFileViper, "", "dns_zone")
	viperconfig.ResetConfBool(dnsFlag, envViper, configFileViper, "", "dns")
	viperconfig.ResetConfInt(minPeers, envViper, configFileViper, "", "min_peers")
	viperconfig.ResetConfString(keyFile, envViper, configFileViper, "", "key")
	viperconfig.ResetConfBool(isArchival, envViper, configFileViper, "", "is_archival")
	viperconfig.ResetConfString(delayCommit, envViper, configFileViper, "", "delay_commit")
	viperconfig.ResetConfString(nodeType, envViper, configFileViper, "", "node_type")
	viperconfig.ResetConfString(networkType, envViper, configFileViper, "", "network_type")
	viperconfig.ResetConfInt(blockPeriod, envViper, configFileViper, "", "block_period")
	viperconfig.ResetConfBool(stakingFlag, envViper, configFileViper, "", "staking")
	viperconfig.ResetConfInt(shardID, envViper, configFileViper, "", "shard_id")
	viperconfig.ResetConfString(blsKeyFile, envViper, configFileViper, "", "blskey_file")
	viperconfig.ResetConfString(blsFolder, envViper, configFileViper, "", "blsfolder")
	viperconfig.ResetConfString(blsPass, envViper, configFileViper, "", "blsPass")
	viperconfig.ResetConfUInt(devnetNumShards, envViper, configFileViper, "", "dn_num_shards")
	viperconfig.ResetConfInt(devnetShardSize, envViper, configFileViper, "", "dn_shard_size")
	viperconfig.ResetConfInt(devnetHarmonySize, envViper, configFileViper, "", "dn_hmy_size")
	viperconfig.ResetConfInt(verbosity, envViper, configFileViper, "", "verbosity")
	viperconfig.ResetConfString(dbDir, envViper, configFileViper, "", "db_dir")
	viperconfig.ResetConfBool(publicRPC, envViper, configFileViper, "", "public_rpc")
	viperconfig.ResetConfInt(doRevertBefore, envViper, configFileViper, "", "do_revert_before")
	viperconfig.ResetConfInt(revertTo, envViper, configFileViper, "", "revert_to")
	viperconfig.ResetConfBool(revertBeacon, envViper, configFileViper, "", "revert_beacon")
	viperconfig.ResetConfString(blacklistPath, envViper, configFileViper, "", "blacklist")
	viperconfig.ResetConfString(webHookYamlPath, envViper, configFileViper, "", "webhook_yaml")
}

func main() {
	// HACK Force usage of go implementation rather than the C based one. Do the right way, see the
	// notes one line 66,67 of https://golang.org/src/net/net.go that say can make the decision at
	// build time.
	os.Setenv("GODEBUG", "netdns=go")

	flag.Var(&p2p.BootNodes, "bootnodes", "a list of bootnode multiaddress (delimited by ,)")
	flag.Parse()

	switch *nodeType {
	case "validator":
	case "explorer":
		break
	default:
		_, _ = fmt.Fprintf(os.Stderr, "Unknown node type: %s\n", *nodeType)
		os.Exit(1)
	}

	nodeconfig.SetPublicRPC(*publicRPC)
	nodeconfig.SetVersion(
		fmt.Sprintf("Harmony (C) 2020. %v, version %v-%v (%v %v)",
			path.Base(os.Args[0]), version, commit, builtBy, builtAt),
	)
	if *versionFlag {
		printVersion()
	}

	switch *networkType {
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
		if *devnetHarmonySize < 0 {
			*devnetHarmonySize = *devnetShardSize
		}
		// TODO (leo): use a passing list of accounts here
		devnetConfig, err := shardingconfig.NewInstance(
			uint32(*devnetNumShards), *devnetShardSize, *devnetHarmonySize, numeric.OneDec(), genesis.HarmonyAccounts, genesis.FoundationalNodeAccounts, nil, shardingconfig.VLBPE)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "ERROR invalid devnet sharding config: %s",
				err)
			os.Exit(1)
		}
		shard.Schedule = shardingconfig.NewFixedSchedule(devnetConfig)
	default:
		_, _ = fmt.Fprintf(os.Stderr, "invalid network type: %#v\n", *networkType)
		os.Exit(2)
	}

	setupViperConfig()

	initSetup()

	if *nodeType == "validator" {
		var err error
		if *stakingFlag {
			err = setupStakingNodeAccount()
		} else {
			err = setupLegacyNodeAccount()
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot set up node account: %s\n", err)
			os.Exit(1)
		}
	}
	if *nodeType == "validator" {
		fmt.Printf("%s mode; node key %s -> shard %d\n",
			map[bool]string{false: "Legacy", true: "Staking"}[*stakingFlag],
			nodeconfig.GetDefaultConfig().ConsensusPriKey.GetPublicKeys().SerializeToHexStr(),
			initialAccounts[0].ShardID)
	}
	if *nodeType != "validator" && *shardID >= 0 {
		for _, initialAccount := range initialAccounts {
			utils.Logger().Info().
				Uint32("original", initialAccount.ShardID).
				Int("override", *shardID).
				Msg("ShardID Override")
			initialAccount.ShardID = uint32(*shardID)
		}
	}

	nodeConfig, err := createGlobalConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR cannot configure node: %s\n", err)
		os.Exit(1)
	}
	currentNode := setupConsensusAndNode(nodeConfig)
	nodeconfig.GetDefaultConfig().ShardID = nodeConfig.ShardID

	// Prepare for graceful shutdown from os signals
	osSignal := make(chan os.Signal)
	signal.Notify(osSignal, os.Interrupt, syscall.SIGTERM)
	go func() {
		for sig := range osSignal {
			if sig == syscall.SIGTERM || sig == os.Interrupt {
				const msg = "Got %s signal. Gracefully shutting down...\n"
				utils.Logger().Printf(msg, sig)
				fmt.Printf(msg, sig)
				currentNode.ShutDown()
			}
		}
	}()

	if nodeConfig.ShardID != shard.BeaconChainShardID {
		utils.Logger().Info().
			Uint32("shardID", currentNode.Blockchain().ShardID()).
			Uint32("shardID", nodeConfig.ShardID).Msg("SupportBeaconSyncing")
		currentNode.SupportBeaconSyncing()
	}

	if uint64(*doRevertBefore) != 0 && uint64(*revertTo) != 0 {
		chain := currentNode.Blockchain()
		if *revertBeacon {
			chain = currentNode.Beaconchain()
		}
		curNum := chain.CurrentBlock().NumberU64()
		if curNum < uint64(*doRevertBefore) && curNum >= uint64(*revertTo) {
			// Remove invalid blocks
			for chain.CurrentBlock().NumberU64() >= uint64(*revertTo) {
				curBlock := chain.CurrentBlock()
				rollbacks := []ethCommon.Hash{curBlock.Hash()}
				chain.Rollback(rollbacks)
				lastSig := curBlock.Header().LastCommitSignature()
				sigAndBitMap := append(lastSig[:], curBlock.Header().LastCommitBitmap()...)
				chain.WriteCommitSig(curBlock.NumberU64()-1, sigAndBitMap)
			}
		}
	}

	startMsg := "==== New Harmony Node ===="
	if *nodeType == "explorer" {
		startMsg = "==== New Explorer Node ===="
	}

	utils.Logger().Info().
		Str("BLSPubKey", nodeConfig.ConsensusPriKey.GetPublicKeys().SerializeToHexStr()).
		Uint32("ShardID", nodeConfig.ShardID).
		Str("ShardGroupID", nodeConfig.GetShardGroupID().String()).
		Str("BeaconGroupID", nodeConfig.GetBeaconGroupID().String()).
		Str("ClientGroupID", nodeConfig.GetClientGroupID().String()).
		Str("Role", currentNode.NodeConfig.Role().String()).
		Str("multiaddress",
			fmt.Sprintf("/ip4/%s/tcp/%s/p2p/%s", *ip, *port, myHost.GetID().Pretty()),
		).
		Msg(startMsg)

	nodeconfig.SetPeerID(myHost.GetID())

	currentNode.SupportSyncing()
	currentNode.ServiceManagerSetup()
	currentNode.RunServices()

	if err := currentNode.StartRPC(*port); err != nil {
		utils.Logger().Warn().
			Err(err).
			Msg("StartRPC failed")
	}

	if err := currentNode.BootstrapConsensus(); err != nil {
		fmt.Println("could not bootstrap consensus", err.Error())
		os.Exit(-1)
	}

	if err := currentNode.Start(); err != nil {
		fmt.Println("could not begin network message handling for node", err.Error())
		os.Exit(-1)
	}
}
