package main

import "testing"

func TestConvertIntoMap(t *testing.T) {
	data := "minh:3,mike:2"
	res := ConvertIntoMap(data)
	if len(res) != 2 {
		t.Errorf("Result should have 2 pairs (key, value)")
	}
	if val, ok := res["minh"]; !ok {
		t.Errorf("Result should contain key minh")
	} else {
		if res["minh"] != 3 {
			t.Errorf("Value of minh should be 3")
		}
	}
	if val, ok := res["mike"]; !ok {
		t.Errorf("Result should contain key mike")
	} else {
		if res["minh"] != 3 {
			t.Errorf("Value of minh should be 2")
		}
	}
}
