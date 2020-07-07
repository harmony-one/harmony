package blsloader

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"

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

// basicBLSLoader loads a single bls key through a key file and passphrase combination.
func loadBasicKeyFromFile(blsKeyFile string, pp passProvider) (*ffibls.SecretKey, error) {
	if pp == nil {
		return nil, errNilPassProvider
	}
	pass, err := pp.getPassphrase(blsKeyFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get password")
	}
	secretKey, err := blsgen.LoadBLSKeyWithPassPhrase(blsKeyFile, pass)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load BLS key")
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

// blsDirLoader is the helper structure
type blsDirLoader struct {
	dirPath   string
	pp        passProvider
	kcp       kmsClientProvider
	resWriter io.Writer

	secretKeys []*ffibls.SecretKey
}

func (loader *blsDirLoader) getKeyFilesFromDir(dir string) (multibls.PrivateKey, error) {
	err := filepath.Walk(dir, loader.processFileWalk)
	if err != nil {
		return multibls.PrivateKey{}, err
	}
	return multibls.PrivateKey{PrivateKey: loader.secretKeys}, nil
}

// error types to be neglected for directory bls loading
var skippingErrs = []error{
	errUnknownExtension,
	errNilPassProvider,
	errNilKMSClientProvider,
}

func (loader *blsDirLoader) processFileWalk(path string, info os.FileInfo, err error) error {
	key, err := loader.loadKeyFromFile(path, info)
	if err != nil {
		if errIsErrors(err, skippingErrs) {
			skipStr := fmt.Sprintf("Skipping [%s]: %v\n", path, err)
			loader.resWriter.Write([]byte(skipStr))
			return nil
		}
		return err
	}
	loader.secretKeys = append(loader.secretKeys, key)
	return nil
}

func (loader *blsDirLoader) loadKeyFromFile(path string, info os.FileInfo) (*ffibls.SecretKey, error) {
	var (
		key *ffibls.SecretKey
		err error
	)
	switch {
	case isBasicKeyFile(info):
		key, err = loadBasicKeyFromFile(path, loader.pp)
	case isKMSKeyFile(info):
		key, err = loadKMSKeyFromFile(path, loader.kcp)
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

// keyFileToPassFile convert a key file base name to passphrase file base name
func keyFileToPassFile(keyFileBase string) string {
	return strings.Trim(keyFileBase, basicKeyExt) + passExt
}

// passFileToKeyFile convert a pass file base name to key file base name
func passFileToKeyFile(passFileBase string) string {
	return strings.Trim(passFileBase, passExt) + basicKeyExt
}

func promptGetPassword(prompt string) (string, error) {
	if !strings.HasSuffix(prompt, ":") {
		prompt += ":"
	}
	fmt.Print(prompt)
	b, err := terminal.ReadPassword(syscall.Stdin)
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
		fmt.Print(prompt)
		response, err := reader.ReadString('\n')
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
