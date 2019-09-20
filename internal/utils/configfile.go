package utils

// this module in utils handles the ini file read/write
import (
	"fmt"
	"strings"

	"gopkg.in/ini.v1"

	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/p2p"
)

// WalletProfile contains a section and key value pair map
type WalletProfile struct {
	Profile   string
	ChainID   string
	Bootnodes []string
	Shards    int
	RPCServer [][]p2p.Peer
}

// ReadWalletProfile reads an ini file and return WalletProfile
func ReadWalletProfile(iniBytes []byte, profile string) (*WalletProfile, error) {
	cfg, err := ini.ShadowLoad(iniBytes)
	if err != nil {
		return nil, err
	}
	config := new(WalletProfile)
	config.Profile = profile

	// get the profile section
	sec, err := cfg.GetSection(profile)
	if err != nil {
		return nil, err
	}
	profile = sec.Name() // sanitized name

	if sec.HasKey("bootnode") {
		config.Bootnodes = sec.Key("bootnode").ValueWithShadows()
	} else {
		return nil, fmt.Errorf("can't find bootnode key")
	}
	if sec.HasKey("chain_id") {
		config.ChainID = sec.Key("chain_id").String()
	} else {
		// backward compatibility; use profile name to determine
		switch profile {
		case "main", "default":
			config.ChainID = params.MainnetChainID.String()
		case "pangaea":
			config.ChainID = params.PangaeaChainID.String()
		default:
			config.ChainID = params.TestnetChainID.String()
		}
	}

	if sec.HasKey("shards") {
		config.Shards = sec.Key("shards").MustInt()
		config.RPCServer = make([][]p2p.Peer, config.Shards)
	} else {
		return nil, fmt.Errorf("can't find shards key")
	}

	for i := 0; i < config.Shards; i++ {
		rpcSec, err := cfg.GetSection(fmt.Sprintf("%s.shard%v.rpc", profile, i))
		if err != nil {
			return nil, err
		}
		rpcKey := rpcSec.Key("rpc").ValueWithShadows()
		for _, key := range rpcKey {
			v := strings.Split(key, ":")
			rpc := p2p.Peer{
				IP:   v[0],
				Port: v[1],
			}
			config.RPCServer[i] = append(config.RPCServer[i], rpc)
		}
	}

	return config, nil

}
