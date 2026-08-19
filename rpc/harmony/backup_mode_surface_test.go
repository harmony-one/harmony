package rpc

import (
	"reflect"
	"testing"
)

// TestBackupModeIsNotOnThePublicSurface checks where SetNodeToBackupMode lives.
// RPC methods are published by reflecting over a service's exported methods, so
// having it on the public blockchain service is what makes it callable on the
// public HTTP and websocket endpoints.
func TestBackupModeIsNotOnThePublicSurface(t *testing.T) {
	const method = "SetNodeToBackupMode"

	if _, found := reflect.TypeOf(&PublicBlockchainService{}).MethodByName(method); found {
		t.Errorf("%s is on the public blockchain service", method)
	}
	if _, found := reflect.TypeOf(&PrivateDebugService{}).MethodByName(method); !found {
		t.Errorf("%s should be on the private debug service", method)
	}
}
