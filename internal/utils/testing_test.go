package utils

import (
	"testing"

	"github.com/ethereum/go-ethereum/log"
	"github.com/golang/mock/gomock"

	mock_utils "github.com/harmony-one/harmony/internal/utils/mock"
)

func TestNewTestLogRedirector(t *testing.T) {
	logger := log.New()
	origHandler := logger.GetHandler()
	tlr := NewTestLogRedirector(logger, t)
	if tlr == nil {
		t.Fatalf("NewTestLogRedirector() = nil")
	}
	if tlr.l != logger {
		t.Errorf("NewTestLogRedirector().l = %v, want %v", tlr.l, logger)
	}
	if tlr.h != origHandler {
		t.Errorf("NewTestLogRedirector().h = %v, want %v", tlr.h, origHandler)
	}
	if tlr.t != t {
		t.Errorf("NewTestLogRedirector().t = %v, want %v", tlr.t, t)
	}
}

func TestTestLogRedirector_Log(t *testing.T) {
	type args struct {
		r *log.Record
	}
	tests := []struct {
		name string
		rec  log.Record
		want string
	}{
		{
			"WithoutContexts",
			log.Record{Lvl: log.LvlDebug, Msg: "hello world"},
			`[dbug] hello world`,
		},
		{
			"WithContexts",
			log.Record{Lvl: log.LvlWarn, Msg: "hello", Ctx: []interface{}{
				"audience", "world",
				"pid", 31337,
			}},
			`[warn] hello, audience="world", pid=31337`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockTestLogger := mock_utils.NewMockTestLogger(ctrl)
			redirector := NewTestLogRedirector(log.New(), mockTestLogger)
			mockTestLogger.EXPECT().Log(tt.want)
			if err := redirector.Log(&tt.rec); err != nil {
				t.Errorf("TestLogRedirector.Log() error = %v, want nil", err)
			}
		})
	}
}

func TestTestLogRedirector_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockTestLogger := mock_utils.NewMockTestLogger(ctrl)
	logger := log.New()
	tlr := NewTestLogRedirector(logger, mockTestLogger)
	mockTestLogger.EXPECT().Log("[info] before close").Times(1)
	mockTestLogger.EXPECT().Log("[info] after close").Times(0)
	mockTestLogger.EXPECT().Log("[info] after idempotent close").Times(0)
	logger.Info("before close")
	if err := tlr.Close(); err != nil {
		t.Errorf("TestLogRedirector.Close() error = %v, want nil", err)
	}
	logger.Info("after close")
	if err := tlr.Close(); err != nil {
		t.Errorf("TestLogRedirector.Close() error = %v, want nil", err)
	}
	logger.Info("after idempotent close")
}
