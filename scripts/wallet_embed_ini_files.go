package main

import "github.com/harmony-one/harmony/pkg/utils"

// Embed the default wallet.ini file into defaultWalletIni string literal constant
func main() {
	utils.EmbedFile("../../../.hmy/wallet.ini", "defaultWalletIni")
}
