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

func (config kmsProviderConfig) validate() error {
	if !config.awsCfgSrcType.isValid() {
		return errors.New("unknown AwsCfgSrcType")
	}
	if config.awsCfgSrcType == AwsCfgSrcFile {
		if !stringIsSet(config.awsConfigFile) {
			return errors.New("config field AwsConfig file must set for AwsCfgSrcFile")
		}
		if !isFile(*config.awsConfigFile) {
			return fmt.Errorf("aws config file not exist %v", *config.awsConfigFile)
		}
	}
	return nil
}

// kmsProvider provide the aws kms client
type kmsProvider interface {
	getKMSClient() (*kms.KMS, error)
}

// lazyKmsProvider provide the kms client with singleton lazy initialization with config get
// from awsConfigGetter for aws credential and regions loading.
type lazyKmsProvider struct {
	acGetter awsConfigGetter

	client *kms.KMS
	err    error
	once   sync.Once
}

// newLazyKmsProvider creates a kmsProvider with the given config
func newLazyKmsProvider(config kmsProviderConfig) (*lazyKmsProvider, error) {
	var acg awsConfigGetter
	switch config.awsCfgSrcType {
	case AwsCfgSrcFile:
		if stringIsSet(config.awsConfigFile) {
			acg = newFileACGetter(*config.awsConfigFile)
		} else {
			acg = newSharedAwsConfigGetter()
		}
	case AwsCfgSrcPrompt:
		acg = newPromptACGetter(defKmsPromptTimeout)
	case AwsCfgSrcShared:
		acg = newSharedAwsConfigGetter()
	default:
		return nil, errors.New("unknown aws config source type")
	}
	return &lazyKmsProvider{
		acGetter: acg,
	}, nil
}

func (provider *lazyKmsProvider) getKMSClient() (*kms.KMS, error) {
	provider.once.Do(func() {
		cfg, err := provider.acGetter.getAwsConfig()
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

// awsConfigGetter provides the aws config. Implemented by
//   sharedACGetter - provide the nil to use shared AWS configuration
//   fileACGetter   - provide the aws config with a json file
//   promptACGetter - provide the config field from prompt with time out
type awsConfigGetter interface {
	getAwsConfig() (*AwsConfig, error)
	String() string
}

// sharedACGetter returns nil for getAwsConfig to use shared aws configurations
type sharedACGetter struct{}

func newSharedAwsConfigGetter() *sharedACGetter {
	return &sharedACGetter{}
}

func (getter *sharedACGetter) getAwsConfig() (*AwsConfig, error) {
	return nil, nil
}

func (getter *sharedACGetter) String() string {
	return "shared aws config"
}

// fileACGetter get aws config through a customized json file
type fileACGetter struct {
	file string
}

func newFileACGetter(file string) *fileACGetter {
	return &fileACGetter{file}
}

func (getter *fileACGetter) getAwsConfig() (*AwsConfig, error) {
	b, err := ioutil.ReadFile(getter.file)
	if err != nil {
		return nil, err
	}
	var cfg AwsConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (getter *fileACGetter) String() string {
	return fmt.Sprintf("file %v", getter.file)
}

// promptACGetter provide a user interactive console for AWS config.
// Four fields are asked:
//    1. AccessKey  	2. SecretKey		3. Region
// Each field is asked with a timeout mechanism.
type promptACGetter struct {
	timeout time.Duration
}

func newPromptACGetter(timeout time.Duration) *promptACGetter {
	return &promptACGetter{
		timeout: timeout,
	}
}

func (getter *promptACGetter) getAwsConfig() (*AwsConfig, error) {
	console.println("Please provide AWS configurations for KMS encoded BLS keys:")
	accessKey, err := getter.prompt("  AccessKey:")
	if err != nil {
		return nil, fmt.Errorf("cannot get aws access key: %v", err)
	}
	secretKey, err := getter.prompt("  SecretKey:")
	if err != nil {
		return nil, fmt.Errorf("cannot get aws secret key: %v", err)
	}
	region, err := getter.prompt("Region:")
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
func (getter *promptACGetter) prompt(hint string) (string, error) {
	var (
		res string
		err error

		finished = make(chan struct{})
		timedOut = time.After(getter.timeout)
	)

	go func() {
		res, err = getter.threadedPrompt(hint)
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

func (getter *promptACGetter) threadedPrompt(hint string) (string, error) {
	console.print(hint)
	return console.readPassword()
}

func (getter *promptACGetter) String() string {
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
