package blsgen

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/pkg/errors"
)

// AwsCfgSrcType is the type of src to load aws config. Four options available:
//
//	AwsCfgSrcNil    - Disable kms decryption
//	AwsCfgSrcFile   - Provide the aws config through a file (json).
//	AwsCfgSrcPrompt - Provide the aws config though prompt.
//	AwsCfgSrcShared - Use the shard aws config (env -> default .aws directory)
type AwsCfgSrcType uint8

const (
	// AwsCfgSrcNil is the nil place holder for AwsCfgSrcType.
	AwsCfgSrcNil AwsCfgSrcType = iota
	// AwsCfgSrcFile instruct reading aws config through a json file.
	AwsCfgSrcFile
	// AwsCfgSrcPrompt use a user interactive prompt to ge aws config.
	AwsCfgSrcPrompt
	// AwsCfgSrcShared use shared AWS config and credentials from env and ~/.aws files.
	AwsCfgSrcShared
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

	provider awsConfigProvider
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
		kd.provider = newFileACProvider(*config.awsConfigFile)
	case AwsCfgSrcPrompt:
		kd.provider = newPromptACProvider(defKmsPromptTimeout)
	case AwsCfgSrcShared:
		kd.provider = newSharedAwsConfigProvider()
	}
}

func (kd *kmsDecrypter) getKMSClient() (*kms.KMS, error) {
	kd.once.Do(func() {
		cfg, err := kd.provider.getAwsConfig()
		if err != nil {
			kd.err = err
			return
		}
		kd.client, kd.err = kmsClientWithConfig(cfg)
	})
	if kd.err != nil {
		return nil, kd.err
	}
	return kd.client, nil
}

// AwsConfig is the config data structure for credentials and region. Used for AWS KMS
// decryption.
type AwsConfig struct {
	AccessKey string `json:"aws-access-key-id"`
	SecretKey string `json:"aws-secret-access-key"`
	Region    string `json:"aws-region"`
	Token     string `json:"aws-token,omitempty"`
}

func (cfg AwsConfig) toAws() *aws.Config {
	cred := credentials.NewStaticCredentials(cfg.AccessKey, cfg.SecretKey, cfg.Token)
	return &aws.Config{
		Region:      aws.String(cfg.Region),
		Credentials: cred,
	}
}

// awsConfigProvider provides the aws config. Implemented by
//
//	sharedACProvider - provide the nil to use shared AWS configuration
//	fileACProvider   - provide the aws config with a json file
//	promptACProvider - provide the config field from prompt with time out
//
// TODO: load aws session set up in a more official way. E.g. session.Opt.SharedConfigFiles,
//
//	profile, env, e.t.c.
type awsConfigProvider interface {
	getAwsConfig() (*AwsConfig, error)
}

// sharedACProvider returns nil for getAwsConfig to use shared aws configurations
type sharedACProvider struct{}

func newSharedAwsConfigProvider() *sharedACProvider {
	return &sharedACProvider{}
}

func (provider *sharedACProvider) getAwsConfig() (*AwsConfig, error) {
	return nil, nil
}

// fileACProvider get aws config through a customized json file
type fileACProvider struct {
	file string
}

func newFileACProvider(file string) *fileACProvider {
	return &fileACProvider{file}
}

func (provider *fileACProvider) getAwsConfig() (*AwsConfig, error) {
	b, err := os.ReadFile(provider.file)
	if err != nil {
		return nil, err
	}
	var cfg AwsConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// promptACProvider provide a user interactive console for AWS config.
// Four fields are asked:
//  1. AccessKey  	2. SecretKey		3. Region
//
// Each field is asked with a timeout mechanism.
type promptACProvider struct {
	timeout time.Duration
}

func newPromptACProvider(timeout time.Duration) *promptACProvider {
	return &promptACProvider{
		timeout: timeout,
	}
}

func (provider *promptACProvider) getAwsConfig() (*AwsConfig, error) {
	console.println("Please provide AWS configurations for KMS encoded BLS keys:")
	accessKey, err := provider.prompt("  AccessKey:")
	if err != nil {
		return nil, fmt.Errorf("cannot get aws access key: %v", err)
	}
	secretKey, err := provider.prompt("  SecretKey:")
	if err != nil {
		return nil, fmt.Errorf("cannot get aws secret key: %v", err)
	}
	region, err := provider.prompt("  Region:")
	if err != nil {
		return nil, fmt.Errorf("cannot get aws region: %v", err)
	}
	return &AwsConfig{
		AccessKey: accessKey,
		SecretKey: secretKey,
		Region:    region,
		Token:     "",
	}, nil
}

// prompt prompt the user to input a string for a certain field with timeout.
func (provider *promptACProvider) prompt(hint string) (string, error) {
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
			console.println("ERROR input time out")
			return "", errors.New("timed out")
		}
	}
}

func (provider *promptACProvider) threadedPrompt(cs consoleItf, hint string) (string, error) {
	cs.print(hint)
	return cs.readPassword()
}

func kmsClientWithConfig(config *AwsConfig) (*kms.KMS, error) {
	if config == nil {
		return getSharedKMSClient()
	}
	return getKMSClientFromConfig(*config)
}

func getSharedKMSClient() (*kms.KMS, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create aws session")
	}
	return kms.New(sess), err
}

func getKMSClientFromConfig(config AwsConfig) (*kms.KMS, error) {
	sess, err := session.NewSession(config.toAws())
	if err != nil {
		return nil, err
	}
	return kms.New(sess), nil
}
