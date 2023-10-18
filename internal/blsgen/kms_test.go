package blsgen

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/ethereum/go-ethereum/common"
	ffi_bls "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/crypto/bls"
)

var TestAwsConfig = AwsConfig{
	AccessKey: "access key",
	SecretKey: "secret key",
	Region:    "region",
}

func TestNewKmsDecrypter(t *testing.T) {
	unitTestDir := filepath.Join(baseTestDir, t.Name())
	testFile := filepath.Join(unitTestDir, "test.json")
	if err := writeAwsConfigFile(testFile, TestAwsConfig); err != nil {
		t.Fatal(err)
	}
	emptyFile := filepath.Join(unitTestDir, "empty.json")

	tests := []struct {
		config      kmsDecrypterConfig
		expProvider awsConfigProvider
		expErr      error
	}{
		{
			config: kmsDecrypterConfig{
				awsCfgSrcType: AwsCfgSrcNil,
			},
			expErr: errors.New("unknown AwsCfgSrcType"),
		},
		{
			config: kmsDecrypterConfig{
				awsCfgSrcType: AwsCfgSrcShared,
			},
			expProvider: &sharedACProvider{},
		},
		{
			config: kmsDecrypterConfig{
				awsCfgSrcType: AwsCfgSrcPrompt,
			},
			expProvider: &promptACProvider{},
		},
		{
			config: kmsDecrypterConfig{
				awsCfgSrcType: AwsCfgSrcFile,
				awsConfigFile: &testFile,
			},
			expProvider: &fileACProvider{},
		},
		{
			config: kmsDecrypterConfig{
				awsCfgSrcType: AwsCfgSrcFile,
			},
			expErr: errors.New("config field AwsConfig file must set for AwsCfgSrcFile"),
		},
		{
			config: kmsDecrypterConfig{
				awsCfgSrcType: AwsCfgSrcFile,
				awsConfigFile: &emptyFile,
			},
			expErr: errors.New("no such file"),
		},
	}
	for i, test := range tests {
		kd, err := newKmsDecrypter(test.config)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
			continue
		}
		if err != nil || test.expErr != nil {
			continue
		}
		gotType := reflect.TypeOf(kd.provider).Elem()
		expType := reflect.TypeOf(test.expProvider).Elem()
		if gotType != expType {
			t.Errorf("Test %v: unexpected aws config provider type: %v / %v",
				i, gotType, expType)
		}
	}
}

func writeAwsConfigFile(file string, config AwsConfig) error {
	b, err := json.Marshal(config)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Dir(file)); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(filepath.Dir(file), 0700)
		} else {
			return err
		}
	}
	return os.WriteFile(file, b, 0700)
}

func TestPromptACProvider_getAwsConfig(t *testing.T) {
	tc := newTestConsole()
	setTestConsole(tc)

	for _, input := range []string{
		TestAwsConfig.AccessKey,
		TestAwsConfig.SecretKey,
		TestAwsConfig.Region,
	} {
		tc.In <- input
	}
	provider := newPromptACProvider(1 * time.Second)
	got, err := provider.getAwsConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*got, TestAwsConfig) {
		t.Errorf("unexpected result %+v / %+v", got, TestAwsConfig)
	}
}

func TestPromptACProvider_prompt(t *testing.T) {
	tests := []struct {
		delay, timeout time.Duration
		expErr         error
	}{
		{
			delay:   100 * time.Microsecond,
			timeout: 1 * time.Second,
			expErr:  nil,
		},
		{
			delay:   2 * time.Second,
			timeout: 1 * time.Second,
			expErr:  errors.New("timed out"),
		},
	}
	for i, test := range tests {
		tc := newTestConsole()
		setTestConsole(tc)

		testInput := "test"
		go func() {
			<-time.After(test.delay)
			tc.In <- testInput
		}()
		provider := newPromptACProvider(test.timeout)
		got, err := provider.prompt("test ask string")

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
			continue
		}
		if err != nil {
			continue
		}
		if got != testInput {
			t.Errorf("Test %v: unexpected prompt result: %v / %v", i, got, testInput)
		}
	}
}

func TestFileACProvider_getAwsConfig(t *testing.T) {
	jsonBytes, err := json.Marshal(TestAwsConfig)
	if err != nil {
		t.Fatal(err)
	}
	unitTestDir := filepath.Join(baseTestDir, t.Name())

	tests := []struct {
		setupFunc func(rootDir string) error
		expConfig AwsConfig
		jsonFile  string
		expErr    error
	}{
		{
			// positive
			setupFunc: func(rootDir string) error {
				jsonFile := filepath.Join(rootDir, "valid.json")
				return writeFile(jsonFile, string(jsonBytes))
			},
			jsonFile:  "valid.json",
			expConfig: TestAwsConfig,
		},
		{
			// no such file
			setupFunc: nil,
			jsonFile:  "empty.json",
			expErr:    errors.New("no such file"),
		},
		{
			// invalid json string
			setupFunc: func(rootDir string) error {
				jsonFile := filepath.Join(rootDir, "invalid.json")
				return writeFile(jsonFile, string(jsonBytes[:len(jsonBytes)-2]))
			},
			jsonFile: "invalid.json",
			expErr:   errors.New("unexpected end of JSON input"),
		},
	}
	for i, test := range tests {
		tcDir := filepath.Join(unitTestDir, fmt.Sprintf("%v", i))
		os.RemoveAll(tcDir)
		os.MkdirAll(tcDir, 0700)
		if test.setupFunc != nil {
			if err := test.setupFunc(tcDir); err != nil {
				t.Fatal(err)
			}
		}

		provider := newFileACProvider(filepath.Join(tcDir, test.jsonFile))
		got, err := provider.getAwsConfig()

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}
		if got == nil || !reflect.DeepEqual(*got, test.expConfig) {
			t.Errorf("Test %v: unexpected AwsConfig: %+v / %+v", i,
				got, test.expConfig)
		}
	}
}

// This is the learning test for kms encryption and decryption. This is just to illustrate
// To successfully run this test, need to set the AWS default configuration and set up kms
// key and replace keyId field.
func TestKMSEncryption(t *testing.T) {
	t.SkipNow()
	client, err := getSharedKMSClient()
	if err != nil {
		t.Fatal(err)
	}
	privHex := testKeys[0].privateKey
	keyID := "26adbb7b-6c46-4763-a7b3-de7ee768890a" // Replace your key ID here

	output, err := client.Encrypt(&kms.EncryptInput{
		KeyId:     &keyID,
		Plaintext: common.Hex2Bytes(privHex),
	})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("encrypted: [%x]\n", output.CiphertextBlob)
	decryted, err := client.Decrypt(&kms.DecryptInput{CiphertextBlob: output.CiphertextBlob})
	if err != nil {
		t.Fatal(err)
	}
	priKey := &ffi_bls.SecretKey{}
	if err = priKey.DeserializeHexStr(hex.EncodeToString(decryted.Plaintext)); err != nil {
		t.Fatal(err)
	}
	pubKey := bls.FromLibBLSPublicKeyUnsafe(priKey.GetPublicKey())
	if hex.EncodeToString(pubKey[:]) != testKeys[0].publicKey {
		t.Errorf("unexpected public key")
	}
}
