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

// MakeGetAccountRangeRequest makes the GetAccountRange request
func MakeGetAccountRangeRequest(root common.Hash, origin common.Hash, limit common.Hash, bytes uint64) *Request {
	return &Request{
		Request: &Request_GetAccountRangeRequest{
			GetAccountRangeRequest: &GetAccountRangeRequest{
				Root:   root[:],
				Origin: origin[:],
				Limit:  limit[:],
				Bytes:  bytes,
			},
		},
	}
}

// MakeGetStorageRangesRequest makes the GetStorageRanges request
func MakeGetStorageRangesRequest(root common.Hash, accounts []common.Hash, origin common.Hash, limit common.Hash, bytes uint64) *Request {
	return &Request{
		Request: &Request_GetStorageRangesRequest{
			GetStorageRangesRequest: &GetStorageRangesRequest{
				Root:     root[:],
				Accounts: hashesToBytes(accounts),
				Origin:   origin[:],
				Limit:    limit[:],
				Bytes:    bytes,
			},
		},
	}
}

// MakeGetByteCodesRequest makes the GetByteCodes request
func MakeGetByteCodesRequest(hashes []common.Hash, bytes uint64) *Request {
	return &Request{
		Request: &Request_GetByteCodesRequest{
			GetByteCodesRequest: &GetByteCodesRequest{
				Hashes: hashesToBytes(hashes),
				Bytes:  bytes,
			},
		},
	}
}

// MakeGetTrieNodesRequest makes the GetTrieNodes request
func MakeGetTrieNodesRequest(root common.Hash, paths []*TrieNodePathSet, bytes uint64) *Request {
	return &Request{
		Request: &Request_GetTrieNodesRequest{
			GetTrieNodesRequest: &GetTrieNodesRequest{
				Root:  root[:],
				Paths: paths,
				Bytes: bytes,
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

// MakeGetAccountRangeResponseMessage makes the GetAccountRangeResponse of Message type
func MakeGetAccountRangeResponseMessage(rid uint64, accounts []*AccountData, proof [][]byte) *Message {
	resp := MakeGetAccountRangeResponse(rid, accounts, proof)
	return makeMessageFromResponse(resp)
}

// MakeGetAccountRangeResponse make the GetAccountRangeResponse of Response type
func MakeGetAccountRangeResponse(rid uint64, accounts []*AccountData, proof [][]byte) *Response {
	return &Response{
		ReqId: rid,
		Response: &Response_GetAccountRangeResponse{
			GetAccountRangeResponse: &GetAccountRangeResponse{
				Accounts: accounts,
				Proof:    proof,
			},
		},
	}
}

// MakeGetStorageRangesResponseMessage makes the GetStorageRangesResponse of Message type
func MakeGetStorageRangesResponseMessage(rid uint64, slots []*StoragesData, proof [][]byte) *Message {
	resp := MakeGetStorageRangesResponse(rid, slots, proof)
	return makeMessageFromResponse(resp)
}

// MakeGetStorageRangesResponse make the GetStorageRangesResponse of Response type
func MakeGetStorageRangesResponse(rid uint64, slots []*StoragesData, proof [][]byte) *Response {
	return &Response{
		ReqId: rid,
		Response: &Response_GetStorageRangesResponse{
			GetStorageRangesResponse: &GetStorageRangesResponse{
				Slots: slots,
				Proof: proof,
			},
		},
	}
}

// MakeGetByteCodesResponseMessage makes the GetByteCodesResponse of Message type
func MakeGetByteCodesResponseMessage(rid uint64, codes [][]byte) *Message {
	resp := MakeGetByteCodesResponse(rid, codes)
	return makeMessageFromResponse(resp)
}

// MakeGetByteCodesResponse make the GetByteCodesResponse of Response type
func MakeGetByteCodesResponse(rid uint64, codes [][]byte) *Response {
	return &Response{
		ReqId: rid,
		Response: &Response_GetByteCodesResponse{
			GetByteCodesResponse: &GetByteCodesResponse{
				Codes: codes,
			},
		},
	}
}

// MakeGetTrieNodesResponseMessage makes the GetTrieNodesResponse of Message type
func MakeGetTrieNodesResponseMessage(rid uint64, nodes [][]byte) *Message {
	resp := MakeGetTrieNodesResponse(rid, nodes)
	return makeMessageFromResponse(resp)
}

// MakeGetTrieNodesResponse make the GetTrieNodesResponse of Response type
func MakeGetTrieNodesResponse(rid uint64, nodes [][]byte) *Response {
	return &Response{
		ReqId: rid,
		Response: &Response_GetTrieNodesResponse{
			GetTrieNodesResponse: &GetTrieNodesResponse{
				Nodes: nodes,
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
