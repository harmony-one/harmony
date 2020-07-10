package blsloader

import (
	"os"
	"path/filepath"
	"strings"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/internal/blsgen"
	"github.com/pkg/errors"
)

func loadBasicKeyWithProvider(blsKeyFile string, pp passProvider) (*bls_core.SecretKey, error) {
	pass, err := pp.getPassphrase(blsKeyFile)
	if err != nil {
		return nil, err
	}
	secretKey, err := blsgen.LoadBLSKeyWithPassPhrase(blsKeyFile, pass)
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
		return errors.New("is directory")
	}
	return nil
}

func checkIsDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return errors.New("is a file")
	}
	return nil
}

func checkIsPassFile(path string) error {
	err := checkIsFile(path)
	if err != nil {
		return err
	}
	if filepath.Ext(path) != passExt {
		return errors.New("should have extension .pass")
	}
	return nil
}

func keyFileToPassFileFull(keyFile string) string {
	return strings.Trim(keyFile, basicKeyExt) + passExt
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
