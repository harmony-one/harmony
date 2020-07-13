package blsloader

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

var TestAwsConfig = AwsConfig{
	AccessKey: "access key",
	SecretKey: "secret key",
	Region:    "region",
}

//func TestNewKmsDecrypter(t *testing.T) {
//	tests := []struct {
//
//	}
//}

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
			continue
		}
		if err != nil {
			continue
		}

		if got == nil || !reflect.DeepEqual(*got, test.expConfig) {
			t.Errorf("Test %v: unexpected AwsConfig: %+v / %+v", i,
				got, test.expConfig)
		}
	}
}
