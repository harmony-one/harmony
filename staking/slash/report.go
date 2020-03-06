package slash

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"gopkg.in/yaml.v2"
)

const (
	// DefaultWebHookPath ..
	DefaultWebHookPath = "staking/slash/webhook.example.yaml"
)

// DoubleSignWebHooks ..
type DoubleSignWebHooks struct {
	WebHooks *struct {
		OnNoticeDoubleSign     string `yaml:"notice-double-sign"`
		OnThisNodeDoubleSigned string `yaml:"this-node-double-signed"`
	} `yaml:"web-hooks"`
}

// NewDoubleSignWebHooksFromPath ..
func NewDoubleSignWebHooksFromPath(yamlPath string) (*DoubleSignWebHooks, error) {
	rawYAML, err := ioutil.ReadFile(yamlPath)
	if err != nil {
		return nil, err
	}
	t := DoubleSignWebHooks{}
	if err := yaml.UnmarshalStrict(rawYAML, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// ReportResult ..
type ReportResult struct {
	Result  string `json:"result"`
	Payload string `json:"payload"`
}

// NewSuccess ..
func NewSuccess(payload string) *ReportResult {
	return &ReportResult{"success", payload}
}

// NewFailure ..
func NewFailure(payload string) *ReportResult {
	return &ReportResult{"failure", payload}
}

// DoPost is a fire and forget helper
func DoPost(url string, record *Record) (*ReportResult, error) {
	payload, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	result, err := ioutil.ReadAll(resp.Body)
	anon := ReportResult{}
	if err := json.Unmarshal(result, &anon); err != nil {
		return nil, err
	}
	return &anon, nil
}
