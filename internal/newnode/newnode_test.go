package newnode

import "testing"

func TestNewNode(t *testing.T) {
	var ip, port string
	ip = "127.0.0.1"
	port = "8080"
	nnode := New(ip, port)

	if nnode.PubK == nil {
		t.Error("new node public key not initialized")
	}

	if nnode.SetInfo {
		t.Error("new node setinfo initialized to true! (should be false)")
	}
}
