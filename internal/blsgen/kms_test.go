package blsgen

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

var (
	AccessKey = "access key"
	SecretKey = "secret key"
	Region    = "region"
)

func TestNewKmsDecrypter(t *testing.T) {
	unitTestDir := filepath.Join(baseTestDir, t.Name())
	testFile := filepath.Join(unitTestDir, "test.json")
	if err := writeFile(testFile, "random string"); err != nil {
		t.Fatal(err)
	}
	emptyFile := filepath.Join(unitTestDir, "empty.json")

	tests := []struct {
		config      kmsDecrypterConfig
		expProvider awsOptProvider
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
			expProvider: &sharedAOProvider{},
		},
		{
			config: kmsDecrypterConfig{
				awsCfgSrcType: AwsCfgSrcPrompt,
			},
			expProvider: &promptAOProvider{},
		},
		{
			config: kmsDecrypterConfig{
				awsCfgSrcType: AwsCfgSrcFile,
				awsConfigFile: &testFile,
			},
			expProvider: &fileAOProvider{},
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

func TestPromptACProvider_getAwsConfig(t *testing.T) {
	tc := newTestConsole()
	setTestConsole(tc)

	for _, input := range []string{
		AccessKey,
		SecretKey,
		Region,
	} {
		tc.In <- input
	}
	provider := newPromptAOProvider(1 * time.Second)
	got, err := provider.getAwsOpt()
	if err != nil {
		t.Fatal(err)
	}
	if *got.Config.Region != Region {
		t.Errorf("unexpected region: %v / %v", got.Config.Region, Region)
	}
}

func TestPromptAOProvider_prompt(t *testing.T) {
	tests := []struct {
		delay, timeout time.Duration
		expErr         error
	}{
		{
			delay:   100 * time.Microsecond,
			timeout: 1000 * time.Microsecond,
			expErr:  nil,
		},
		{
			delay:   2000 * time.Microsecond,
			timeout: 1000 * time.Microsecond,
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
		provider := newPromptAOProvider(test.timeout)
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
