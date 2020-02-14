package slash

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/harmony-one/bls/ffi/go/bls"
	"gopkg.in/yaml.v2"
)

// DoubleSignWebHooks ..
type DoubleSignWebHooks struct {
	WebHooks *struct {
		OnNoticeDoubleSign     string `yaml:"notice-double-sign"`
		OnThisNodeDoubleSigned string `yaml:"this-node-double-signed"`
	} `yaml:"web-hooks"`
	Malicious *struct {
		Trigger *struct {
			PublicKeys        []string `yaml:"list"`
			DoubleSignNodeURL string   `yaml:"double-sign"`
		} `yaml:"trigger"`
	} `yaml:"malicious"`
}

// Contains ..
func (h *DoubleSignWebHooks) Contains(key *bls.PublicKey) bool {
	hex := key.SerializeToHexStr()
	for _, key := range h.Malicious.Trigger.PublicKeys {
		if hex == key {
			return true
		}
	}
	return false
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

// NewMaliciousHandler ..
func NewMaliciousHandler(cb func()) {
	http.HandleFunc("/trigger-next-double-sign", func(w http.ResponseWriter, r *http.Request) {
		cb()
	})
	if err := http.ListenAndServe(":7777", nil); err != nil {
		fmt.Println("why this died", err.Error())
	}
}
