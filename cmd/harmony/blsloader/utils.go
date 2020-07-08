package blsloader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/internal/blsgen"
)

var (
	errUnknownExtension     = errors.New("unknown extension")
	errNilPassProvider      = errors.New("no source for password")
	errNilKMSClientProvider = errors.New("no source for KMS provider")
)

// loadBasicKey loads a single bls key through a key file and passphrase combination.
// The passphrase is provided by a slice of passProviders.
func loadBasicKey(blsKeyFile string, pps []passProvider) (*bls_core.SecretKey, error) {
	if len(pps) == 0 {
		return nil, errNilPassProvider
	}
	for _, pp := range pps {
		secretKey, err := loadBasicKeyWithProvider(blsKeyFile, pp)
		if err != nil {
			console.println(err)
		}
		return secretKey, nil
	}
	return nil, fmt.Errorf("failed to load bls key %v", blsKeyFile)
}

func loadBasicKeyWithProvider(blsKeyFile string, pp passProvider) (*bls_core.SecretKey, error) {
	pass, err := pp.getPassphrase(blsKeyFile)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get passphrase from %s", pp.toStr())
	}
	secretKey, err := blsgen.LoadBLSKeyWithPassPhrase(blsKeyFile, pass)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to decrypt bls key with %s\n", pp.toStr())
	}
	return secretKey, nil
}

// loadKmsKeyFromFile loads a single KMS BLS key from file
func loadKmsKeyFromFile(blsKeyFile string, kcp kmsClientProvider) (*bls_core.SecretKey, error) {
	if kcp == nil {
		return nil, errNilKMSClientProvider
	}
	client, err := kcp.getKMSClient()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get KMS client")
	}
	secretKey, err := blsgen.LoadAwsCMKEncryptedBLSKey(blsKeyFile, client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load KMS BLS key")
	}
	return secretKey, nil
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func isBasicKeyFile(path string) bool {
	exist := isFile(path)
	if !exist {
		return false
	}
	return filepath.Ext(path) == basicKeyExt
}

func isKMSKeyFile(path string) bool {
	exist := isFile(path)
	if !exist {
		return false
	}
	return filepath.Ext(path) == kmsKeyExt
}

func keyFileToPassFileBase(keyFileBase string) string {
	return strings.Trim(keyFileBase, basicKeyExt) + passExt
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
		response = strings.ToLower(response)

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
