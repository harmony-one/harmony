package slash

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"gopkg.in/yaml.v2"
)

// DoubleSignWebHooks ..
type DoubleSignWebHooks struct {
	WebHooks *struct {
		OnNoticeDoubleSign     string `yaml:"notice-double-sign"`
		OnThisNodeDoubleSigned string `yaml:"this-node-double-signed"`
	} `yaml:"web-hooks"`
	Malicious *struct {
		ValidatorPublicKey string `yaml:"validator-public-key"`
		TriggerDoubleSign  string `yaml:"trigger-double-sign"`
	} `yaml:"malicious"`
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
func NewMaliciousHandler(url string) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("triggered yay")
	}
	http.HandleFunc("/trigger-next-double-sign", handler)

	if err := http.ListenAndServe(":7777", nil); err != nil {
		fmt.Println("why this died", err.Error())
	}

}
