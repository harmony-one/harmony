package wallet

// this module in utils handles the ini file read/write
import (
	"fmt"
	"os"
	"strings"

	ini "gopkg.in/ini.v1"

	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/p2p"
)

// Profile contains a section and key value pair map
type Profile struct {
	Profile   string
	ChainID   string
	Bootnodes []string
	Shards    int
	RPCServer [][]p2p.Peer
	Network   string
}

// ReadProfile reads an ini file and return Profile
func ReadProfile(iniBytes []byte, profile string) (*Profile, error) {
	cfg, err := ini.ShadowLoad(iniBytes)
	if err != nil {
		return nil, err
	}
	config := new(Profile)
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
	if sec.HasKey("network") {
		config.Network = sec.Key("network").String()
	} else {
		config.Network = "devnet"
	}
	if sec.HasKey("chain_id") {
		config.ChainID = sec.Key("chain_id").String()
	} else {
		// backward compatibility; use profile name to determine
		// (deprecated; require chain_id after 2010-01).
		switch profile {
		case "main", "default":
			config.ChainID = params.MainnetChainID.String()
		case "pangaea":
			config.ChainID = params.PangaeaChainID.String()
		default:
			config.ChainID = params.TestnetChainID.String()
		}
		_, _ = fmt.Fprintf(os.Stderr,
			"NOTICE: Chain ID not found in config profile, assuming %s; "+
				"please add \"chain_id = %s\" to section [%s] of wallet.ini "+
				"before 2020-01\n",
			config.ChainID, config.ChainID, profile)
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
