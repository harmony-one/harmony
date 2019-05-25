package utils

import (
	"fmt"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"
)

// AskForPassphrase return passphrase using password input
func AskForPassphrase(prompt string) string {
	fmt.Printf(prompt)
	bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		panic("read password error")
	}
	password := string(bytePassword)
	fmt.Println()

	return password
}
