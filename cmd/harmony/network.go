package main

import (
	"fmt"

	"github.com/harmony-one/harmony/internal/cli"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/spf13/cobra"
)

type networkConfig struct {
	NetworkType string
	BootNodes   []string

	LegacySyncing bool // if true, use LegacySyncingPeerProvider
	DNSZone       string
	DNSPort       int
}

var networkFlags = []cli.Flag{
	networkTypeFlag,
	bootNodeFlag,
	dnsZoneFlag,
	dnsPortFlag,
	legacyDNSZoneFlag,
	legacyDNSPortFlag,
	legacyDNSFlag,
	legacyNetworkTypeFlag,
}

var (
	networkTypeFlag = cli.StringFlag{
		Name:      "network",
		Shorthand: "n",
		DefValue:  "mainnet",
		Usage:     "network to join (mainnet, testnet, pangaea, localnet, partner, stressnet, devnet)",
	}
	bootNodeFlag = cli.StringSliceFlag{
		Name:  "bootnodes",
		Usage: "a list of bootnode multiaddress (delimited by ,)",
	}
	dnsZoneFlag = cli.StringSliceFlag{
		Name:  "dns.zone",
		Usage: "use peers from the zone for state syncing",
	}
	dnsPortFlag = cli.IntFlag{
		Name:     "dns.port",
		DefValue: defDNSPort,
		Usage:    "port of dns node",
	}
	legacyDNSZoneFlag = cli.StringFlag{
		Name:       "dns_zone",
		Usage:      "use peers from the zone for state syncing",
		Deprecated: "use --dns.zone",
	}
	legacyDNSPortFlag = cli.IntFlag{
		Name:       "dns_port",
		Usage:      "port of dns node",
		Deprecated: "use --dns.zone",
	}
	legacyDNSFlag = cli.BoolFlag{
		Name:       "dns",
		DefValue:   true,
		Usage:      "use dns for syncing",
		Deprecated: "equivalent to --dns.zone t.hmny.io",
	}
	legacyNetworkTypeFlag = cli.StringFlag{
		Name:       "network_type",
		Usage:      "network to join (mainnet, testnet, pangaea, localnet, partner, stressnet, devnet)",
		Deprecated: "use --network",
	}
)

func getNetworkType(cmd *cobra.Command) (nodeconfig.NetworkType, error) {
	var (
		raw string
		err error
	)
	if cmd.Flags().Changed(legacyNetworkTypeFlag.Name) {
		raw, err = cmd.Flags().GetString(legacyNetworkTypeFlag.Name)
	} else {
		raw, err = cmd.Flags().GetString(networkTypeFlag.Name)
	}
	if err != nil {
		return "", err
	}

	nt := parseNetworkType(raw)
	if len(nt) == 0 {
		return "", fmt.Errorf("unrecognized network type: %v", nt)
	}
	return nt, nil
}

func parseNetworkType(nt string) nodeconfig.NetworkType {
	switch nt {
	case "mainnet":
		return nodeconfig.Mainnet
	case "testnet":
		return nodeconfig.Testnet
	case "pangaea", "staking", "stk":
		return nodeconfig.Pangaea
	case "partner":
		return nodeconfig.Partner
	case "stressnet", "stress", "stn":
		return nodeconfig.Stressnet
	case "localnet":
		return nodeconfig.Localnet
	case "devnet", "dev":
		return nodeconfig.Devnet
	default:
		return ""
	}
}

func applyNetworkFlags(cmd *cobra.Command, cfg *parsedConfig) {
	fs := cmd.Flags()

	if fs.Changed(bootNodeFlag.Name) {
		cfg.Network.BootNodes, _ = fs.GetStringSlice(bootNodeFlag.Name)
	}

	if fs.Changed(dnsZoneFlag.Name) {
		cfg.Network.DNSZone, _ = fs.GetString(dnsZoneFlag.Name)
	} else if fs.Changed(legacyDNSZoneFlag.Name) {
		cfg.Network.DNSZone, _ = fs.GetString(legacyDNSZoneFlag.Name)
	} else if fs.Changed(legacyDNSFlag.Name) {
		val, _ := fs.GetBool(legacyDNSFlag.Name)
		if val {
			cfg.Network.DNSZone = mainnetDnsZone
		} else {
			cfg.Network.LegacySyncing = true
		}
	}

	if fs.Changed(dnsPortFlag.Name) {
		cfg.Network.DNSPort, _ = fs.GetInt(dnsZoneFlag.Name)
	} else if fs.Changed(legacyDNSPortFlag.Name) {
		cfg.Network.DNSPort, _ = fs.GetInt(legacyDNSPortFlag.Name)
	}
}

func getDefaultNetworkConfig(nt nodeconfig.NetworkType) networkConfig {
	bn := nodeconfig.GetDefaultBootNodes(nt)
	zone := nodeconfig.GetDefaultDNSZone(nt)
	port := nodeconfig.GetDefaultDNSPort(nt)
	return networkConfig{
		NetworkType: string(nt),
		BootNodes:   bn,
		DNSZone:     zone,
		DNSPort:     port,
	}
}
