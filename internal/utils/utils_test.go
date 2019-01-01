package utils

import "testing"

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
