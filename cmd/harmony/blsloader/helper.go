package blsloader

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ffibls "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/internal/blsgen"
	"github.com/harmony-one/harmony/multibls"
	"github.com/pkg/errors"
)

var (
	errUnknownExtension     = errors.New("unknown extension")
	errNilPassProvider      = errors.New("no source for password")
	errNilKMSClientProvider = errors.New("no source for KMS provider")
)

// loadBasicKey loads a single bls key through a key file and passphrase combination.
// The passphrase is provided by a slice of passProviders.
func loadBasicKey(blsKeyFile string, pps []passProvider) (*ffibls.SecretKey, error) {
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

func loadBasicKeyWithProvider(blsKeyFile string, pp passProvider) (*ffibls.SecretKey, error) {
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

// loadKMSKeyFromFile loads a single KMS BLS key from file
func loadKMSKeyFromFile(blsKeyFile string, kcp kmsClientProvider) (*ffibls.SecretKey, error) {
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

// blsDirLoadHelper is the helper structure for loading bls keys in a directory
type blsDirLoadHelper struct {
	dirPath string
	pps     []passProvider
	kcp     kmsClientProvider
	// result field
	secretKeys []*ffibls.SecretKey
}

func (helper *blsDirLoadHelper) getKeyFilesFromDir() (multibls.PrivateKey, error) {
	err := filepath.Walk(helper.dirPath, helper.processFileWalk)
	if err != nil {
		return multibls.PrivateKey{}, err
	}
	return multibls.PrivateKey{PrivateKey: helper.secretKeys}, nil
}

func (helper *blsDirLoadHelper) processFileWalk(path string, info os.FileInfo, err error) error {
	key, err := helper.loadKeyFromFile(path, info)
	if err != nil {
		if !errIsErrors(err, helper.skippingErrors()) {
			// unexpected error, return the error and break the file walk loop
			return err
		}
		// expected error. Skipping these files
		skipStr := fmt.Sprintf("Skipping [%s]: %v\n", path, err)
		console.println(skipStr)
		return nil
	}
	helper.secretKeys = append(helper.secretKeys, key)
	return nil
}

// errors to be neglected for directory bls loading
func (helper *blsDirLoadHelper) skippingErrors() []error {
	return []error{
		errUnknownExtension,
		errNilPassProvider,
		errNilKMSClientProvider,
	}
}

func (helper *blsDirLoadHelper) loadKeyFromFile(path string, info os.FileInfo) (*ffibls.SecretKey, error) {
	var (
		key *ffibls.SecretKey
		err error
	)
	switch {
	case isBasicKeyFile(info):
		key, err = loadBasicKey(path, helper.pps)
	case isKMSKeyFile(info):
		key, err = loadKMSKeyFromFile(path, helper.kcp)
	default:
		err = errUnknownExtension
	}
	return key, err
}

// errIsErrors return whether the err is one of the errs
func errIsErrors(err error, errs []error) bool {
	for _, targetErr := range errs {
		if errors.Is(err, targetErr) {
			return true
		}
	}
	return false
}

// isBasicKeyFile return whether the given file is a bls file
func isBasicKeyFile(info os.FileInfo) bool {
	if info.IsDir() {
		return false
	}
	if !strings.HasSuffix(info.Name(), basicKeyExt) {
		return false
	}
	return true
}

// isKMSKeyFile returns whether the given file is a kms key file
func isKMSKeyFile(info os.FileInfo) bool {
	if info.IsDir() {
		return false
	}
	if !strings.HasSuffix(info.Name(), kmsKeyExt) {
		return false
	}
	return true
}

// isPassphraseFile returns whether the given file is a pass file
func isPassFile(info os.FileInfo) bool {
	if info.IsDir() {
		return false
	}
	if !strings.HasSuffix(info.Name(), passExt) {
		return false
	}
	return true
}

// keyFileToPassFile convert a key file base name to passphrase file base name
func keyFileToPassFile(keyFileBase string) string {
	return strings.Trim(keyFileBase, basicKeyExt) + passExt
}

func promptGetPassword(prompt string) (string, error) {
	if !strings.HasSuffix(prompt, ":") {
		prompt += ":"
	}
	console.print(prompt)
	b, err := console.readPassword()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

const yesNoPrompt = "[y/n]: "

func promptYesNo(prompt string) (bool, error) {
	if !strings.HasSuffix(prompt, yesNoPrompt) {
		prompt = prompt + yesNoPrompt
	}
	reader := bufio.NewReader(os.Stdin)
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
