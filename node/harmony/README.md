Node struct is the core entity that represents a network node participating in the Harmony protocol.

A node is the main message handler to all kinds of protocol messages such as consensus message, block sync, transactions etc. A node contains all the necessary references to other objects (e.g. blockchain object and consensus object) to handle the incoming messages.