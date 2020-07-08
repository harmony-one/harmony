package blsloader

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/pkg/errors"
)

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

// kmsProviderConfig is the data structure of kmsClientProvider config
type kmsProviderConfig struct {
	awsCfgSrcType AwsCfgSrcType
	awsConfigFile *string
}

func (cfg kmsProviderConfig) validate() error {
	if !cfg.awsCfgSrcType.isValid() {
		return errors.New("unknown AwsCfgSrcType")
	}
	if cfg.awsCfgSrcType == AwsCfgSrcFile {
		if !stringIsSet(cfg.awsConfigFile) {
			return errors.New("config field AwsConfig file must set for AwsCfgSrcFile")
		}
		if !isFile(*cfg.awsConfigFile) {
			return fmt.Errorf("aws config file not exist %v", *cfg.awsConfigFile)
		}
	}
	return nil
}

// kmsClientProvider provides the kms client. Implemented by
//   baseKMSProvider - abstract implementation
//   sharedKMSProvider - provide the client with default .aws folder
//   fileKMSProvider - provide the aws config with a json file
//   promptKMSProvider - provide the config field from prompt with time out
type kmsClientProvider interface {
	// getKMSClient returns the KMSClient of the kmsClientProvider with lazy loading.
	getKMSClient() (*kms.KMS, error)

	// toStr return the string presentation of kmsClientProvider
	toStr() string
}

type getAwsCfgFunc func() (*AwsConfig, error)

// baseKMSProvider provide the kms client with singleton initialization through
// function getConfig for aws credential and regions loading.
type baseKMSProvider struct {
	client *kms.KMS
	err    error
	once   sync.Once

	getAWSConfig getAwsCfgFunc
}

func (provider *baseKMSProvider) getKMSClient() (*kms.KMS, error) {
	provider.once.Do(func() {
		cfg, err := provider.getAWSConfig()
		if err != nil {
			provider.err = err
			return
		}
		provider.client, provider.err = kmsClientWithConfig(cfg)
	})
	if provider.err != nil {
		return nil, provider.err
	}
	return provider.client, nil
}

func (provider *baseKMSProvider) toStr() string {
	return "not implemented"
}

// sharedKMSProvider provide the kms session with the default aws config
// locates in directory $HOME/.aws/config
type sharedKMSProvider struct {
	baseKMSProvider
}

func newSharedKmsProvider() *sharedKMSProvider {
	provider := &sharedKMSProvider{baseKMSProvider{}}
	provider.baseKMSProvider.getAWSConfig = provider.getAWSConfig
	return provider
}

// TODO(Jacky): set getAwsConfig into a function, not bind with structure
func (provider *sharedKMSProvider) getAWSConfig() (*AwsConfig, error) {
	return nil, nil
}

func (provider *sharedKMSProvider) toStr() string {
	return "shared aws config"
}

// fileKMSProvider provide the kms session from a file with json data of structure
// AwsConfig
type fileKMSProvider struct {
	baseKMSProvider

	file string
}

func newFileKmsProvider(file string) *fileKMSProvider {
	provider := &fileKMSProvider{
		baseKMSProvider: baseKMSProvider{},
		file:            file,
	}
	provider.baseKMSProvider.getAWSConfig = provider.getAWSConfig
	return provider
}

func (provider *fileKMSProvider) getAWSConfig() (*AwsConfig, error) {
	b, err := ioutil.ReadFile(provider.file)
	if err != nil {
		return nil, err
	}
	var cfg *AwsConfig
	if err := json.Unmarshal(b, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (provider *fileKMSProvider) toStr() string {
	return fmt.Sprintf("file %v", provider.file)
}

// promptKMSProvider provide a user interactive console for AWS config.
// Three fields are asked:
//    1. AccessKey  	2. SecretKey		3. Region
// Each field is asked with a timeout mechanism.
type promptKMSProvider struct {
	baseKMSProvider

	timeout time.Duration
}

func newPromptKmsProvider(timeout time.Duration) *promptKMSProvider {
	provider := &promptKMSProvider{
		baseKMSProvider: baseKMSProvider{},
		timeout:         timeout,
	}
	provider.baseKMSProvider.getAWSConfig = provider.getAWSConfig
	return provider
}

func (provider *promptKMSProvider) getAWSConfig() (*AwsConfig, error) {
	console.println("Please provide AWS configurations for KMS encoded BLS keys:")
	accessKey, err := provider.prompt("  AccessKey:")
	if err != nil {
		return nil, fmt.Errorf("cannot get aws access key: %v", err)
	}
	secretKey, err := provider.prompt("  SecretKey:")
	if err != nil {
		return nil, fmt.Errorf("cannot get aws secret key: %v", err)
	}
	region, err := provider.prompt("Region:")
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
func (provider *promptKMSProvider) prompt(hint string) (string, error) {
	var (
		res string
		err error

		finished = make(chan struct{})
		timedOut = time.After(provider.timeout)
	)

	go func() {
		res, err = provider.threadedPrompt(hint)
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

func (provider *promptKMSProvider) threadedPrompt(hint string) (string, error) {
	console.print(hint)
	return console.readPassword()
}

func (provider *promptKMSProvider) toStr() string {
	return "prompt"
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
