package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCallRequest_UnmarshalFromInterface(t *testing.T) {
	args := map[string]interface{}{
		"to":        "0x08AE1abFE01aEA60a47663bCe0794eCCD5763c19",
		"block_num": 370000,
	}
	callRequest := &CallRequest{}
	err := callRequest.UnmarshalFromInterface(args)
	if err != nil {
		t.Fatal(err.Error())
	}
	assert.Equal(t, callRequest.To.String(), "0x08AE1abFE01aEA60a47663bCe0794eCCD5763c19")
	assert.Equal(t, callRequest.BlockNum, int64(370000))
}

func TestGetCodeRequest_UnmarshalFromInterface(t *testing.T) {
	args := map[string]interface{}{
		"addr":      "0x08AE1abFE01aEA60a47663bCe0794eCCD5763c19",
		"block_num": 370000,
	}
	getCodeRequest := &GetCodeRequest{}
	err := getCodeRequest.UnmarshalFromInterface(args)
	if err != nil {
		t.Fatal(err.Error())
	}
	assert.Equal(t, getCodeRequest.Addr, "0x08AE1abFE01aEA60a47663bCe0794eCCD5763c19")
	assert.Equal(t, getCodeRequest.BlockNum, int64(370000))
}

func TestGetStorageAtRequest_UnmarshalFromInterface(t *testing.T) {
	args := map[string]interface{}{
		"addr":      "0x295a70b2de5e3953354a6a8344e616ed314d7251",
		"key":       "0x0",
		"block_num": 370000,
	}
	getStorageAtRequest := &GetStorageAtRequest{}
	err := getStorageAtRequest.UnmarshalFromInterface(args)
	if err != nil {
		t.Fatal(err.Error())
	}
	assert.Equal(t, getStorageAtRequest.Addr, "0x295a70b2de5e3953354a6a8344e616ed314d7251")
	assert.Equal(t, getStorageAtRequest.Key, "0x0")
	assert.Equal(t, getStorageAtRequest.BlockNum, int64(370000))
}
