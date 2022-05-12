//go:generate solc -o . --abi CXMessageReceiver.sol
//go:generate abigen --abi CXMessageReceiver.abi --pkg cxmessagereceiver --type CXMessageReceiver --out CXMessageReceiver.go

package cxmessagereceiver
