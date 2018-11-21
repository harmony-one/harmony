package newnode

import (
	"reflect"
	"testing"
)

func TestSerializeDeserialize(t *testing.T) {
	node := New("127.0.0.1", "8080")
	serializedNode := node.SerializeNode()
	deserializedNode := DeserializeNode(serializedNode)
	if !reflect.DeepEqual(node, deserializedNode) {
		t.Errorf("Original node and the deserialized node are not equal.")
	}
}
