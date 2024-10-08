package rpc

import (
	"bytes"

	jsoniter "github.com/json-iterator/go"
)

// StructuredResponse type of RPCs
type StructuredResponse = map[string]interface{}

// NewStructuredResponse creates a structured response from the given input
func NewStructuredResponse(input interface{}) (StructuredResponse, error) {
	var objMap StructuredResponse
	var jsonIter = jsoniter.ConfigCompatibleWithStandardLibrary
	dat, err := jsonIter.Marshal(input)
	if err != nil {
		return nil, err
	}
	d := jsonIter.NewDecoder(bytes.NewReader(dat))
	d.UseNumber()
	err = d.Decode(&objMap)
	if err != nil {
		return nil, err
	}
	return objMap, nil
}
