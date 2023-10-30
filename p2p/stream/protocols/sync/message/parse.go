package message

import (
	"fmt"

	"github.com/pkg/errors"
)

// ResponseError is the error from an error response
type ResponseError struct {
	msg string
}

// Error is the error string of ResponseError
func (err *ResponseError) Error() string {
	return fmt.Sprintf("[RESPONSE] %v", err.msg)
}

// GetBlockNumberResponse parse the message to GetBlockNumberResponse
func (msg *Message) GetBlockNumberResponse() (*GetBlockNumberResponse, error) {
	resp := msg.GetResp()
	if resp == nil {
		return nil, errors.New("not response message")
	}
	if errResp := resp.GetErrorResponse(); errResp != nil {
		return nil, &ResponseError{errResp.Error}
	}
	bnResp := resp.GetGetBlockNumberResponse()
	if bnResp == nil {
		return nil, errors.New("not GetBlockNumber response")
	}
	return bnResp, nil
}

// GetBlockHashesResponse parse the message to GetBlockHashesResponse
func (msg *Message) GetBlockHashesResponse() (*GetBlockHashesResponse, error) {
	resp := msg.GetResp()
	if resp == nil {
		return nil, errors.New("not response message")
	}
	if errResp := resp.GetErrorResponse(); errResp != nil {
		return nil, &ResponseError{errResp.Error}
	}
	ghResp := resp.GetGetBlockHashesResponse()
	if ghResp == nil {
		return nil, errors.New("not GetBlockHashesResponse")
	}
	return ghResp, nil
}

// GetBlocksByNumberResponse parse the message to GetBlocksByNumberResponse
func (msg *Message) GetBlocksByNumberResponse() (*GetBlocksByNumResponse, error) {
	resp := msg.GetResp()
	if resp == nil {
		return nil, errors.New("not response message")
	}
	if errResp := resp.GetErrorResponse(); errResp != nil {
		return nil, &ResponseError{errResp.Error}
	}
	gbResp := resp.GetGetBlocksByNumResponse()
	if gbResp == nil {
		return nil, errors.New("not GetBlocksByNumResponse")
	}
	return gbResp, nil
}

// GetBlocksByHashesResponse parse the message to GetBlocksByHashesResponse
func (msg *Message) GetBlocksByHashesResponse() (*GetBlocksByHashesResponse, error) {
	resp := msg.GetResp()
	if resp == nil {
		return nil, errors.New("not response message")
	}
	if errResp := resp.GetErrorResponse(); errResp != nil {
		return nil, &ResponseError{errResp.Error}
	}
	gbResp := resp.GetGetBlocksByHashesResponse()
	if gbResp == nil {
		return nil, errors.New("not GetBlocksByHashesResponse")
	}
	return gbResp, nil
}

// GetReceiptsResponse parse the message to GetReceiptsResponse
func (msg *Message) GetReceiptsResponse() (*GetReceiptsResponse, error) {
	resp := msg.GetResp()
	if resp == nil {
		return nil, errors.New("not response message")
	}
	if errResp := resp.GetErrorResponse(); errResp != nil {
		return nil, &ResponseError{errResp.Error}
	}
	grResp := resp.GetGetReceiptsResponse()
	if grResp == nil {
		return nil, errors.New("not GetGetReceiptsResponse")
	}
	return grResp, nil
}

// GetNodeDataResponse parse the message to GetNodeDataResponse
func (msg *Message) GetNodeDataResponse() (*GetNodeDataResponse, error) {
	resp := msg.GetResp()
	if resp == nil {
		return nil, errors.New("not response message")
	}
	if errResp := resp.GetErrorResponse(); errResp != nil {
		return nil, &ResponseError{errResp.Error}
	}
	gnResp := resp.GetGetNodeDataResponse()
	if gnResp == nil {
		return nil, errors.New("not GetGetNodeDataResponse")
	}
	return gnResp, nil
}

// GetAccountRangesResponse parse the message to GetAccountRangesResponse
func (msg *Message) GetAccountRangesResponse() (*GetAccountRangeResponse, error) {
	resp := msg.GetResp()
	if resp == nil {
		return nil, errors.New("not response message")
	}
	if errResp := resp.GetErrorResponse(); errResp != nil {
		return nil, &ResponseError{errResp.Error}
	}
	gnResp := resp.GetGetAccountRangeResponse()
	if gnResp == nil {
		return nil, errors.New("not GetGetAccountRangeResponse")
	}
	return gnResp, nil
}

// GetStorageRangesResponse parse the message to GetStorageRangesResponse
func (msg *Message) GetStorageRangesResponse() (*GetStorageRangesResponse, error) {
	resp := msg.GetResp()
	if resp == nil {
		return nil, errors.New("not response message")
	}
	if errResp := resp.GetErrorResponse(); errResp != nil {
		return nil, &ResponseError{errResp.Error}
	}
	gnResp := resp.GetGetStorageRangesResponse()
	if gnResp == nil {
		return nil, errors.New("not GetGetStorageRangesResponse")
	}
	return gnResp, nil
}

// GetByteCodesResponse parse the message to GetByteCodesResponse
func (msg *Message) GetByteCodesResponse() (*GetByteCodesResponse, error) {
	resp := msg.GetResp()
	if resp == nil {
		return nil, errors.New("not response message")
	}
	if errResp := resp.GetErrorResponse(); errResp != nil {
		return nil, &ResponseError{errResp.Error}
	}
	gnResp := resp.GetGetByteCodesResponse()
	if gnResp == nil {
		return nil, errors.New("not GetByteCodesResponse")
	}
	return gnResp, nil
}

// GetTrieNodesResponse parse the message to GetTrieNodesResponse
func (msg *Message) GetTrieNodesResponse() (*GetTrieNodesResponse, error) {
	resp := msg.GetResp()
	if resp == nil {
		return nil, errors.New("not response message")
	}
	if errResp := resp.GetErrorResponse(); errResp != nil {
		return nil, &ResponseError{errResp.Error}
	}
	gnResp := resp.GetGetTrieNodesResponse()
	if gnResp == nil {
		return nil, errors.New("not GetTrieNodesResponse")
	}
	return gnResp, nil
}
