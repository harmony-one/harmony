package blsgen

import (
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/pkg/errors"
)

// AwsConfigSrcType is the type of src to load aws config. Four options available:
//  AwsCfgSrcNil    - Disable kms decryption
//  AwsCfgSrcFile   - Provide the aws config through a file (json).
//  AwsCfgSrcPrompt - Provide the aws config though prompt.
//  AwsCfgSrcShared - Use the shard aws config (env -> default .aws directory)
type AwsCfgSrcType uint8

const (
	AwsCfgSrcNil    AwsCfgSrcType = iota // nil place holder.
	AwsCfgSrcFile                        // through a config file (json)
	AwsCfgSrcPrompt                      // through console prompt.
	AwsCfgSrcShared                      // through shared aws config
)

func (srcType AwsCfgSrcType) isValid() bool {
	switch srcType {
	case AwsCfgSrcFile, AwsCfgSrcPrompt, AwsCfgSrcShared:
		return true
	default:
		return false
	}
}

// kmsDecrypterConfig is the data structure of kmsClientProvider config
type kmsDecrypterConfig struct {
	awsCfgSrcType AwsCfgSrcType
	awsConfigFile *string
}

// kmsDecrypter provide the kms client with singleton lazy initialization with config get
// from awsConfigProvider for aws credential and regions loading.
type kmsDecrypter struct {
	config kmsDecrypterConfig

	provider awsOptProvider
	client   *kms.KMS
	err      error
	once     sync.Once
}

// newKmsDecrypter creates a kmsDecrypter with the given config
func newKmsDecrypter(config kmsDecrypterConfig) (*kmsDecrypter, error) {
	kd := &kmsDecrypter{config: config}
	if err := kd.validateConfig(); err != nil {
		return nil, err
	}
	kd.makeACProvider()
	return kd, nil
}

// extension returns the kms key file extension
func (kd *kmsDecrypter) extension() string {
	return kmsKeyExt
}

// decryptFile decrypt a kms key file to a secret key
func (kd *kmsDecrypter) decryptFile(keyFile string) (*bls_core.SecretKey, error) {
	kms, err := kd.getKMSClient()
	if err != nil {
		return nil, err
	}
	return LoadAwsCMKEncryptedBLSKey(keyFile, kms)
}

func (kd *kmsDecrypter) validateConfig() error {
	config := kd.config
	if !config.awsCfgSrcType.isValid() {
		return errors.New("unknown AwsCfgSrcType")
	}
	if config.awsCfgSrcType == AwsCfgSrcFile {
		if !stringIsSet(config.awsConfigFile) {
			return errors.New("config field AwsConfig file must set for AwsCfgSrcFile")
		}
		if err := checkIsFile(*config.awsConfigFile); err != nil {
			return err
		}
	}
	return nil
}

func (kd *kmsDecrypter) makeACProvider() {
	config := kd.config
	switch config.awsCfgSrcType {
	case AwsCfgSrcFile:
		kd.provider = newFileAOProvider(*config.awsConfigFile)
	case AwsCfgSrcPrompt:
		kd.provider = newPromptAOProvider(defKmsPromptTimeout)
	case AwsCfgSrcShared:
		kd.provider = newSharedAOProvider()
	}
}

func (kd *kmsDecrypter) getKMSClient() (*kms.KMS, error) {
	kd.once.Do(func() {
		opt, err := kd.provider.getAwsOpt()
		if err != nil {
			kd.err = err
			return
		}
		kd.client, kd.err = kmsClientWithOpt(opt)
	})
	if kd.err != nil {
		return nil, kd.err
	}
	return kd.client, nil
}

// awsOptProvider provides the aws session option. Implemented by
//   sharedAOProvider - provide the nil to use shared AWS configuration
//   fileAOProvider   - provide the aws config with a json file
//   promptAOProvider - provide the config field from prompt with time out
type awsOptProvider interface {
	getAwsOpt() (session.Options, error)
}

// sharedAOProvider returns the shared aws config
type sharedAOProvider struct{}

func newSharedAOProvider() *sharedAOProvider {
	return &sharedAOProvider{}
}

func (provider *sharedAOProvider) getAwsOpt() (session.Options, error) {
	return session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}, nil
}

// fileAOProvider get aws config through a customized json file
type fileAOProvider struct {
	file string
}

func newFileAOProvider(file string) *fileAOProvider {
	return &fileAOProvider{file}
}

func (provider *fileAOProvider) getAwsOpt() (session.Options, error) {
	return session.Options{
		SharedConfigFiles: []string{provider.file},
		SharedConfigState: session.SharedConfigEnable,
	}, nil
}

// promptAOProvider provide a user interactive console for AWS config.
// Four fields are asked:
//    1. AccessKey  	2. SecretKey		3. Region
// Each field is asked with a timeout mechanism.
type promptAOProvider struct {
	timeout time.Duration
}

func newPromptAOProvider(timeout time.Duration) *promptAOProvider {
	return &promptAOProvider{
		timeout: timeout,
	}
}

func (provider *promptAOProvider) getAwsOpt() (session.Options, error) {
	console.println("Please provide AWS configurations for KMS encoded BLS keys:")
	accessKey, err := provider.prompt("  AccessKey:")
	if err != nil {
		return session.Options{}, fmt.Errorf("cannot get aws access key: %v", err)
	}
	secretKey, err := provider.prompt("  SecretKey:")
	if err != nil {
		return session.Options{}, fmt.Errorf("cannot get aws secret key: %v", err)
	}
	region, err := provider.prompt("Region:")
	if err != nil {
		return session.Options{}, fmt.Errorf("cannot get aws region: %v", err)
	}
	return session.Options{
		Config: aws.Config{
			Region:      aws.String(region),
			Credentials: credentials.NewStaticCredentials(accessKey, secretKey, ""),
		},
		SharedConfigState: session.SharedConfigEnable,
	}, nil
}

// prompt prompt the user to input a string for a certain field with timeout.
func (provider *promptAOProvider) prompt(hint string) (string, error) {
	var (
		res string
		err error

		finished = make(chan struct{})
		timedOut = time.After(provider.timeout)
	)

	cs := console
	go func() {
		res, err = provider.threadedPrompt(cs, hint)
		close(finished)
	}()

	for {
		select {
		case <-finished:
			return res, err
		case <-timedOut:
			return "", errors.New("timed out")
		}
	}
}

func (provider *promptAOProvider) threadedPrompt(cs consoleItf, hint string) (string, error) {
	cs.print(hint)
	return cs.readPassword()
}

func kmsClientWithOpt(opt session.Options) (*kms.KMS, error) {
	sess, err := session.NewSessionWithOptions(opt)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create aws session")
	}
	return kms.New(sess), err
}
