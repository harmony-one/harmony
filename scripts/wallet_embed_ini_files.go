package main

import "github.com/harmony-one/harmony/internal/utils"

// Embed the default wallet.ini file into defaultWalletIni string literal constant
func main() {
	utils.EmbedFile("../../../.hmy/wallet.ini", "defaultWalletIni")
}
