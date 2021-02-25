package sttypes

import "testing"

func BenchmarkProtoIDToProtoSpec(b *testing.B) {
	stid := ProtoID("harmony/sync/unitest/0/1.0.1")
	for i := 0; i != b.N; i++ {
		ProtoIDToProtoSpec(stid)
	}
}
