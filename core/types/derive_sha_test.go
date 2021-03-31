package types

import "testing"

func TestBloomxxx(t *testing.T) {
	r := NewReceipt([]byte("helloworld"), false, 1)
	t.Log(r)
	//DeriveSha
}
