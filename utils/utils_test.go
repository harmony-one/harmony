package utils

import "testing"

func TestConvertIntoInts(t *testing.T) {
	data := "1,2,3,4"
	res := ConvertIntoInts(data)
	if len(res) != 4 {
		t.Errorf("Array length parsed incorrect.")
	}

	for id, value := range res {
		if value != id+1 {
			t.Errorf("Parsing incorrect.")
		}
	}
}

func TestConvertFixedDataIntoByteArray(t *testing.T) {
	res := ConvertFixedDataIntoByteArray(int16(3))
	if len(res) != 2 {
		t.Errorf("Conversion incorrect.")
	}
	res = ConvertFixedDataIntoByteArray(int32(3))
	if len(res) != 4 {
		t.Errorf("Conversion incorrect.")
	}
}
