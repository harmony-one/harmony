BootNode struct is the core entity that represents a bootstrap node participating in the Harmony protocol.

New nodes in a p2p network often make their initial connection to the p2p network through a set of nodes known as boot nodes. Information (e.g. addresses) about these boot nodes is e.g. embedded in an application binary or provided as a configuration option.

The boot nodes serve as an entry point, providing a list of other nodes in the network to newcomers. After connecting to the boot nodes, the new node can connect to those other nodes in the network, thereby no longer relying on the boot nodes.