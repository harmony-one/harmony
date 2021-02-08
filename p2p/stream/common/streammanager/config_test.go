package streammanager

import "testing"

func TestApplyOptions(t *testing.T) {
	opts := []Option{
		WithDiscBatch(1),
		WithHardLoCap(1),
		WithSoftLoCap(1),
		WithHiCap(1),
	}
	expCfg := Config{
		HardLoCap: 1,
		SoftLoCap: 1,
		HiCap:     1,
		DiscBatch: 1,
	}

	defConfigCpy := defConfig
	sm := newStreamManager(testProtoID, nil, nil, opts...)
	if sm.config != expCfg {
		t.Errorf("unexpected config")
	}
	if defConfig != defConfigCpy {
		t.Errorf("defConfig value changed")
	}
}
