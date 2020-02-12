package slash

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"gopkg.in/yaml.v2"
)

// DoubleSignWebHooks ..
type DoubleSignWebHooks struct {
	WebHooks struct {
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

// DoPost is a fire and forget helper
func DoPost(url string, record *Record) error {
	payload, err := json.Marshal(record)

	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	return resp.Body.Close()
}
