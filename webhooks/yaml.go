package webhooks

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"

	"gopkg.in/yaml.v2"
)

const (
	// DefaultWebHookPath ..
	DefaultWebHookPath = "webhooks/webhook.example.yaml"
)

// AvailabilityHooks ..
type AvailabilityHooks struct {
	OnDroppedBelowThreshold string `yaml:"on-dropped-below-threshold"`
}

// DoubleSignWebHooks ..
type DoubleSignWebHooks struct {
	OnNoticeDoubleSign string `yaml:"on-notice-double-sign"`
}

// BadBlockHooks ..
type BadBlockHooks struct {
	OnCannotCommit string `yaml:"on-cannot-commit-block"`
}

// Hooks ..
type Hooks struct {
	Slashing       *DoubleSignWebHooks `yaml:"slashing-hooks"`
	Availability   *AvailabilityHooks  `yaml:"availability-hooks"`
	ProtocolIssues *BadBlockHooks      `yaml:"protocol-hooks"`
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
func DoPost(url string, record interface{}) (*ReportResult, error) {
	payload, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	anon := ReportResult{}
	if err := json.Unmarshal(result, &anon); err != nil {
		return nil, err
	}
	return &anon, nil
}

// NewWebHooksFromPath ..
func NewWebHooksFromPath(yamlPath string) (*Hooks, error) {
	rawYAML, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, err
	}
	t := Hooks{}
	if err := yaml.UnmarshalStrict(rawYAML, &t); err != nil {
		return nil, err
	}
	return &t, nil
}
