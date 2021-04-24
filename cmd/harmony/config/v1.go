package config

import (
	"fmt"

	"github.com/pelletier/go-toml"
)

const v1TOMLConfigVersion = "1.0.4"

// harmonyConfigV1 contains all the configs user can set for running harmony binary for versions <= 1.0.4.
type harmonyConfigV1 struct {
	Version    string
	General    generalConfigV1
	Network    networkConfigV1
	P2P        p2pConfigV1
	HTTP       httpConfigV1
	WS         wsConfigV1
	RPCOpt     rpcOptConfigV1
	BLSKeys    blsConfigV1
	TxPool     txPoolConfigV1
	Pprof      pprofConfigV1
	Log        logConfigV1
	Sync       syncConfigV1
	Sys        *sysConfigV1        `toml:",omitempty"`
	Consensus  *consensusConfigV1  `toml:",omitempty"`
	Devnet     *devnetConfigV1     `toml:",omitempty"`
	Revert     *revertConfigV1     `toml:",omitempty"`
	Legacy     *legacyConfigV1     `toml:",omitempty"`
	Prometheus *prometheusConfigV1 `toml:",omitempty"`
}

type networkConfigV1 struct {
	NetworkType string
	BootNodes   []string

	LegacySyncing bool // if true, use LegacySyncingPeerProvider
	DNSZone       string
	DNSPort       int
}

type p2pConfigV1 struct {
	Port         int
	IP           string
	KeyFile      string
	DHTDataStore *string `toml:",omitempty"`
}

type generalConfigV1 struct {
	NodeType         string
	NoStaking        bool
	ShardID          int
	IsArchival       bool
	IsBeaconArchival bool
	IsOffline        bool
	DataDir          string
}

type consensusConfigV1 struct {
	MinPeers     int
	AggregateSig bool
}

type blsConfigV1 struct {
	KeyDir   string
	KeyFiles []string
	MaxKeys  int

	PassEnabled    bool
	PassSrcType    string
	PassFile       string
	SavePassphrase bool

	KMSEnabled       bool
	KMSConfigSrcType string
	KMSConfigFile    string
}

type txPoolConfigV1 struct {
	BlacklistFile string
}

type pprofConfigV1 struct {
	Enabled    bool
	ListenAddr string
}

type logConfigV1 struct {
	Folder     string
	FileName   string
	RotateSize int
	Verbosity  int
	Context    *logContextV1 `toml:",omitempty"`
}

type logContextV1 struct {
	IP   string
	Port int
}

type sysConfigV1 struct {
	NtpServer string
}

type httpConfigV1 struct {
	Enabled        bool
	IP             string
	Port           int
	RosettaEnabled bool
	RosettaPort    int
}

type wsConfigV1 struct {
	Enabled bool
	IP      string
	Port    int
}

type rpcOptConfigV1 struct {
	DebugEnabled bool // Enables PrivateDebugService APIs, including the EVM tracer
}

type devnetConfigV1 struct {
	NumShards   int
	ShardSize   int
	HmyNodeSize int
}

// TODO: make `revert` to a separate command
type revertConfigV1 struct {
	RevertBeacon bool
	RevertTo     int
	RevertBefore int
}

type legacyConfigV1 struct {
	WebHookConfig         *string `toml:",omitempty"`
	TPBroadcastInvalidTxn *bool   `toml:",omitempty"`
}

type prometheusConfigV1 struct {
	Enabled    bool
	IP         string
	Port       int
	EnablePush bool
	Gateway    string
}

type syncConfigV1 struct {
	// TODO: Remove this bool after stream sync is fully up.
	Downloader     bool // start the sync downloader client
	LegacyServer   bool // provide the gRPC sync protocol server
	LegacyClient   bool // aside from stream sync protocol, also run gRPC client to get blocks
	Concurrency    int  // concurrency used for stream sync protocol
	MinPeers       int  // minimum streams to start a sync task.
	InitStreams    int  // minimum streams in bootstrap to start sync loop.
	DiscSoftLowCap int  // when number of streams is below this value, spin discover during check
	DiscHardLowCap int  // when removing stream, num is below this value, spin discovery immediately
	DiscHighCap    int  // upper limit of streams in one sync protocol
	DiscBatch      int  // size of each discovery
}

func loadHarmonyConfigV1(b []byte) (HarmonyConfig, error) {
	var oldCfg harmonyConfigV1
	if err := toml.Unmarshal(b, &oldCfg); err != nil {
		return HarmonyConfig{}, fmt.Errorf("parsing config v1: %w", err)
	}
	cfg := HarmonyConfig{
		Version: oldCfg.Version,
		General: GeneralConfig{
			NodeType:         oldCfg.General.NodeType,
			NoStaking:        oldCfg.General.NoStaking,
			ShardID:          oldCfg.General.ShardID,
			IsArchival:       oldCfg.General.IsArchival,
			IsBeaconArchival: oldCfg.General.IsBeaconArchival,
			IsOffline:        oldCfg.General.IsOffline,
			DataDir:          oldCfg.General.DataDir,
		},
		Network: NetworkConfig{
			NetworkType: oldCfg.Network.NetworkType,
			BootNodes:   oldCfg.Network.BootNodes,
		},
		P2P: P2PConfig{
			Port:         oldCfg.P2P.Port,
			IP:           oldCfg.P2P.IP,
			KeyFile:      oldCfg.P2P.KeyFile,
			DHTDataStore: oldCfg.P2P.DHTDataStore,
		},
		HTTP: HTTPConfig{
			Enabled:        oldCfg.HTTP.Enabled,
			IP:             oldCfg.HTTP.IP,
			Port:           oldCfg.HTTP.Port,
			RosettaEnabled: oldCfg.HTTP.RosettaEnabled,
			RosettaPort:    oldCfg.HTTP.RosettaPort,
		},
		WS: WSConfig{
			Enabled: oldCfg.WS.Enabled,
			IP:      oldCfg.WS.IP,
			Port:    oldCfg.WS.Port,
		},
		RPCOpt: RPCOptConfig{
			DebugEnabled: oldCfg.RPCOpt.DebugEnabled,
		},
		BLSKeys: BLSConfig{
			KeyDir:           oldCfg.BLSKeys.KeyDir,
			KeyFiles:         oldCfg.BLSKeys.KeyFiles,
			MaxKeys:          oldCfg.BLSKeys.MaxKeys,
			PassEnabled:      oldCfg.BLSKeys.PassEnabled,
			PassSrcType:      oldCfg.BLSKeys.PassSrcType,
			PassFile:         oldCfg.BLSKeys.PassFile,
			SavePassphrase:   oldCfg.BLSKeys.SavePassphrase,
			KMSEnabled:       oldCfg.BLSKeys.KMSEnabled,
			KMSConfigSrcType: oldCfg.BLSKeys.KMSConfigSrcType,
			KMSConfigFile:    oldCfg.BLSKeys.KMSConfigFile,
		},
		TxPool: TxPoolConfig{
			BlacklistFile: oldCfg.TxPool.BlacklistFile,
		},
		Pprof: PprofConfig{
			Enabled:    oldCfg.Pprof.Enabled,
			ListenAddr: oldCfg.Pprof.ListenAddr,
		},
		Log: LogConfig{
			Folder:     oldCfg.Log.Folder,
			FileName:   oldCfg.Log.FileName,
			RotateSize: oldCfg.Log.RotateSize,
			Verbosity:  oldCfg.Log.Verbosity,
		},
		Sync: SyncConfig{
			Downloader:     oldCfg.Sync.Downloader,
			Concurrency:    oldCfg.Sync.Concurrency,
			MinPeers:       oldCfg.Sync.MinPeers,
			InitStreams:    oldCfg.Sync.InitStreams,
			DiscSoftLowCap: oldCfg.Sync.DiscSoftLowCap,
			DiscHardLowCap: oldCfg.Sync.DiscHardLowCap,
			DiscHighCap:    oldCfg.Sync.DiscHighCap,
			DiscBatch:      oldCfg.Sync.DiscBatch,
		},
		DNSSync: DNSSyncConfig{
			LegacySyncing: oldCfg.Network.LegacySyncing,
			DNSZone:       oldCfg.Network.DNSZone,
			DNSPort:       oldCfg.Network.DNSPort,
			LegacyServer:  oldCfg.Sync.LegacyServer,
			LegacyClient:  oldCfg.Sync.LegacyClient,
		},
	}
	if oldCfg.Log.Context != nil {
		cfg.Log.Context = &LogContext{
			IP:   oldCfg.Log.Context.IP,
			Port: oldCfg.Log.Context.Port,
		}
	}
	if oldCfg.Sys != nil {
		cfg.Sys = &SysConfig{
			NtpServer: oldCfg.Sys.NtpServer,
		}
	}
	if oldCfg.Consensus != nil {
		cfg.Consensus = &ConsensusConfig{
			MinPeers:     oldCfg.Consensus.MinPeers,
			AggregateSig: oldCfg.Consensus.AggregateSig,
		}
	}
	if oldCfg.Devnet != nil {
		cfg.Devnet = &DevnetConfig{
			NumShards:   oldCfg.Devnet.NumShards,
			ShardSize:   oldCfg.Devnet.ShardSize,
			HmyNodeSize: oldCfg.Devnet.HmyNodeSize,
		}
	}
	if oldCfg.Revert != nil {
		cfg.Revert = &RevertConfig{
			RevertBeacon: oldCfg.Revert.RevertBeacon,
			RevertTo:     oldCfg.Revert.RevertTo,
			RevertBefore: oldCfg.Revert.RevertBefore,
		}
	}
	if oldCfg.Legacy != nil {
		cfg.Legacy = &LegacyConfig{
			WebHookConfig:         cfg.Legacy.WebHookConfig,
			TPBroadcastInvalidTxn: cfg.Legacy.TPBroadcastInvalidTxn,
		}
	}
	if cfg.Prometheus != nil {
		cfg.Prometheus = &PrometheusConfig{
			Enabled:    oldCfg.Prometheus.Enabled,
			IP:         oldCfg.Prometheus.IP,
			Port:       oldCfg.Prometheus.Port,
			EnablePush: oldCfg.Prometheus.EnablePush,
			Gateway:    oldCfg.Prometheus.Gateway,
		}
	}
	return cfg, nil
}
