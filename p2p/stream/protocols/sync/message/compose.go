package message

import (
	"github.com/ethereum/go-ethereum/common"
)

// MakeGetBlockNumberRequest makes the GetBlockNumber Request
func MakeGetBlockNumberRequest() *Request {
	return &Request{
		Request: &Request_GetBlockNumberRequest{
			GetBlockNumberRequest: &GetBlockNumberRequest{},
		},
	}
}

// MakeGetBlockHashesRequest makes GetBlockHashes Request
func MakeGetBlockHashesRequest(bns []uint64) *Request {
	return &Request{
		Request: &Request_GetBlockHashesRequest{
			GetBlockHashesRequest: &GetBlockHashesRequest{
				Nums: bns,
			},
		},
	}
}

// MakeGetBlocksByNumRequest makes the GetBlockByNumber request
func MakeGetBlocksByNumRequest(bns []uint64) *Request {
	return &Request{
		Request: &Request_GetBlocksByNumRequest{
			GetBlocksByNumRequest: &GetBlocksByNumRequest{
				Nums: bns,
			},
		},
	}
}

// MakeGetBlockByHashesRequest makes the GetBlocksByHashes request
func MakeGetBlocksByHashesRequest(hashes []common.Hash) *Request {
	return &Request{
		Request: &Request_GetBlocksByHashesRequest{
			GetBlocksByHashesRequest: &GetBlocksByHashesRequest{
				BlockHashes: hashesToBytes(hashes),
			},
		},
	}
}

// MakeGetNodeDataRequest makes the GetNodeData request
func MakeGetNodeDataRequest(hashes []common.Hash) *Request {
	return &Request{
		Request: &Request_GetNodeDataRequest{
			GetNodeDataRequest: &GetNodeDataRequest{
				NodeHashes: hashesToBytes(hashes),
			},
		},
	}
}

// MakeGetReceiptsRequest makes the GetReceipts request
func MakeGetReceiptsRequest(hashes []common.Hash) *Request {
	return &Request{
		Request: &Request_GetReceiptsRequest{
			GetReceiptsRequest: &GetReceiptsRequest{
				BlockHashes: hashesToBytes(hashes),
			},
		},
	}
}

// MakeErrorResponse makes the error response
func MakeErrorResponseMessage(rid uint64, err error) *Message {
	resp := MakeErrorResponse(rid, err)
	return makeMessageFromResponse(resp)
}

// MakeErrorResponse makes the error response as a response
func MakeErrorResponse(rid uint64, err error) *Response {
	return &Response{
		ReqId: rid,
		Response: &Response_ErrorResponse{
			&ErrorResponse{
				Error: err.Error(),
			},
		},
	}
}

// MakeGetBlockNumberResponseMessage makes the GetBlockNumber response message
func MakeGetBlockNumberResponseMessage(rid uint64, bn uint64) *Message {
	resp := MakeGetBlockNumberResponse(rid, bn)
	return makeMessageFromResponse(resp)
}

// MakeGetBlockNumberResponse makes the GetBlockNumber response
func MakeGetBlockNumberResponse(rid uint64, bn uint64) *Response {
	return &Response{
		ReqId: rid,
		Response: &Response_GetBlockNumberResponse{
			GetBlockNumberResponse: &GetBlockNumberResponse{
				Number: bn,
			},
		},
	}
}

// MakeGetBlockHashesResponseMessage makes the GetBlockHashes message
func MakeGetBlockHashesResponseMessage(rid uint64, hs []common.Hash) *Message {
	resp := MakeGetBlockHashesResponse(rid, hs)
	return makeMessageFromResponse(resp)
}

// MakeGetBlockHashesResponse makes the GetBlockHashes response
func MakeGetBlockHashesResponse(rid uint64, hs []common.Hash) *Response {
	return &Response{
		ReqId: rid,
		Response: &Response_GetBlockHashesResponse{
			GetBlockHashesResponse: &GetBlockHashesResponse{
				Hashes: hashesToBytes(hs),
			},
		},
	}
}

// MakeGetBlocksByNumResponseMessage makes the GetBlocksByNumResponse of Message type
func MakeGetBlocksByNumResponseMessage(rid uint64, blocksBytes, sigs [][]byte) *Message {
	resp := MakeGetBlocksByNumResponse(rid, blocksBytes, sigs)
	return makeMessageFromResponse(resp)
}

// MakeGetBlocksByNumResponseMessage make the GetBlocksByNumResponse of Response type
func MakeGetBlocksByNumResponse(rid uint64, blocksBytes, sigs [][]byte) *Response {
	return &Response{
		ReqId: rid,
		Response: &Response_GetBlocksByNumResponse{
			GetBlocksByNumResponse: &GetBlocksByNumResponse{
				BlocksBytes: blocksBytes,
				CommitSig:   sigs,
			},
		},
	}
}

// MakeGetBlocksByHashesResponseMessage makes the GetBlocksByHashesResponse of Message type
func MakeGetBlocksByHashesResponseMessage(rid uint64, blocksBytes, sigs [][]byte) *Message {
	resp := MakeGetBlocksByHashesResponse(rid, blocksBytes, sigs)
	return makeMessageFromResponse(resp)
}

// MakeGetBlocksByHashesResponse make the GetBlocksByHashesResponse of Response type
func MakeGetBlocksByHashesResponse(rid uint64, blocksBytes [][]byte, sigs [][]byte) *Response {
	return &Response{
		ReqId: rid,
		Response: &Response_GetBlocksByHashesResponse{
			GetBlocksByHashesResponse: &GetBlocksByHashesResponse{
				BlocksBytes: blocksBytes,
				CommitSig:   sigs,
			},
		},
	}
}

// MakeGetNodeDataResponseMessage makes the GetNodeDataResponse of Message type
func MakeGetNodeDataResponseMessage(rid uint64, nodeData [][]byte) *Message {
	resp := MakeGetNodeDataResponse(rid, nodeData)
	return makeMessageFromResponse(resp)
}

// MakeGetNodeDataResponse make the GetNodeDataResponse of Response type
func MakeGetNodeDataResponse(rid uint64, nodeData [][]byte) *Response {
	return &Response{
		ReqId: rid,
		Response: &Response_GetNodeDataResponse{
			GetNodeDataResponse: &GetNodeDataResponse{
				DataBytes: nodeData,
			},
		},
	}
}

// MakeGetReceiptsResponseMessage makes the GetReceiptsResponse of Message type
func MakeGetReceiptsResponseMessage(rid uint64, receipts map[uint64]*Receipts) *Message {
	resp := MakeGetReceiptsResponse(rid, receipts)
	return makeMessageFromResponse(resp)
}

// MakeGetReceiptsResponse make the GetReceiptsResponse of Response type
func MakeGetReceiptsResponse(rid uint64, receipts map[uint64]*Receipts) *Response {
	return &Response{
		ReqId: rid,
		Response: &Response_GetReceiptsResponse{
			GetReceiptsResponse: &GetReceiptsResponse{
				Receipts: receipts,
			},
		},
	}
}

// MakeMessageFromRequest makes a message from the request
func MakeMessageFromRequest(req *Request) *Message {
	return &Message{
		ReqOrResp: &Message_Req{
			Req: req,
		},
	}
}

func makeMessageFromResponse(resp *Response) *Message {
	return &Message{
		ReqOrResp: &Message_Resp{
			Resp: resp,
		},
	}
}

func hashesToBytes(hashes []common.Hash) [][]byte {
	res := make([][]byte, 0, len(hashes))

	for _, h := range hashes {
		b := make([]byte, common.HashLength)
		copy(b, h[:])
		res = append(res, b)
	}
	return res
}

func bytesToHashes(bs [][]byte) []common.Hash {
	res := make([]common.Hash, len(bs))

	for _, b := range bs {
		var h common.Hash
		copy(h[:], b)
		res = append(res, h)
	}
	return res
}
