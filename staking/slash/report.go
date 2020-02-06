package slash

import (
	"io/ioutil"

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
