package main

import "github.com/spf13/cobra"

var networkFlags *networkCmdFlags

type networkCmdFlags struct {
	networkType string
	bootNodes   []string
	dnsZone     string
	dnsPort     int

	// legacy flags
	legacyDNSZone     string
	legacyDNSPort     string
	legacyDNSFlag     bool
	legacyNetworkType string
}

func registerNetworkFlags(cmd *cobra.Command) error {
	flags := cmd.Flags()
	flags.StringVarP(&networkFlags.networkType, "network", "n", "mainnet", "network to join (mainnet, testnet, pangaea, localnet, partner, stressnet, devnet")
	flags.StringSliceVarP(&networkFlags.bootNodes, "bootnodes", "", []string{}, "a list of bootnode multiaddress (delimited by ,)")
	flags.StringVarP(&networkFlags.dnsZone, "dns-zone", "", "", "use peers from the zone for state syncing")
	flags.IntVarP(&networkFlags.dnsPort, "dns-port", "", 9000, "port of dns node")

	flags.StringVarP(&networkFlags.legacyDNSZone, "dns_zone", "", "", "use peers from the zone for state syncing (deprecated: use dns-zone)")
	flags.StringVarP(&networkFlags.legacyDNSPort, "dns_port", "", "", "port of dns node (deprecated: use --dns-port)")
	flags.BoolVarP(&networkFlags.legacyDNSFlag, "dns", "", true, "deprecated: equivalent to -dns_zone t.hmny.io")
	flags.StringVarP(&networkFlags.legacyNetworkType, "network_type", "", "", "network to join (deprecated: use --network-type)")

	legacyFlags := []string{"dns_zone", "dns_port", "dns", "network_type"}
	for _, legacyFlag := range legacyFlags {
		if err := flags.MarkHidden(legacyFlag); err != nil {
			return err
		}
	}
	return nil
}

func parseNetworkFlagToConfig(cmd *cobra.Command, cfg *parsedConfig) error {
	flags := cmd.Flags()
	if flags.Changed("network_type") {
		cfg.Network.NetworkType = flags.GetBool()
	}
}
