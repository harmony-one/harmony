package blsgen

import (
	"fmt"
	"os"
	"strings"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"
)

func loadBasicKeyWithProvider(blsKeyFile string, pp passProvider) (*bls_core.SecretKey, error) {
	pass, err := pp.getPassphrase(blsKeyFile)
	if err != nil {
		return nil, err
	}
	secretKey, err := LoadBLSKeyWithPassPhrase(blsKeyFile, pass)
	if err != nil {
		return nil, err
	}
	return secretKey, nil
}

func checkIsFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%v is directory", path)
	}
	return nil
}

func checkIsDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%v is a file", path)
	}
	return nil
}

func keyFileToPassFileFull(keyFile string) string {
	return strings.TrimSuffix(keyFile, basicKeyExt) + passExt
}

func promptGetPassword(prompt string) (string, error) {
	if !strings.HasSuffix(prompt, ":") {
		prompt += ":"
	}
	console.print(prompt)
	return console.readPassword()
}

const yesNoPrompt = "[y/n]: "

func promptYesNo(prompt string) (bool, error) {
	if !strings.HasSuffix(prompt, yesNoPrompt) {
		prompt = prompt + yesNoPrompt
	}
	for {
		console.print(prompt)
		response, err := console.readln()
		if err != nil {
			return false, err
		}
		response = strings.TrimSpace(strings.ToLower(response))

		if response == "y" || response == "yes" {
			return true, nil
		} else if response == "n" || response == "no" {
			return false, nil
		}
	}
}

func stringIsSet(val *string) bool {
	return val != nil && *val != ""
}
