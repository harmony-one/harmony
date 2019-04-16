package utils

import (
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/log"
	"github.com/golang/mock/gomock"

	matchers "github.com/harmony-one/harmony/gomock_matchers"
	"github.com/harmony-one/harmony/internal/utils/mock_log"
)

//go:generate mockgen -destination mock_log/logger.go github.com/ethereum/go-ethereum/log Logger
//go:generate mockgen -destination mock_log/handler.go github.com/ethereum/go-ethereum/log Handler

const (
	thisPkg  = "github.com/harmony-one/harmony/internal/utils"
	thisFile = "logging_test.go"
)

func testWithCallerSkip0(t *testing.T, skip int, name string, line int) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	want := log.Root()
	logger := mock_log.NewMockLogger(ctrl)
	logger.EXPECT().New(matchers.Slice{
		"funcName", thisPkg + "." + name,
		"funcFile", thisFile,
		"funcLine", line,
	}).Return(want)
	if got := WithCallerSkip(logger, skip); !reflect.DeepEqual(got, want) {
		t.Errorf("WithCallerSkip() = %v, want %v", got, want)
	}
}

func testWithCallerSkip1(t *testing.T, skip int, name string, line int) {
	testWithCallerSkip0(t, skip, name, line)
}

func testWithCallerSkip2(t *testing.T, skip int, name string, line int) {
	testWithCallerSkip1(t, skip, name, line)
}

func TestWithCallerSkip(t *testing.T) {
	t.Run("0", func(t *testing.T) {
		testWithCallerSkip2(t, 0, "testWithCallerSkip0", 32)
	})
	t.Run("1", func(t *testing.T) {
		testWithCallerSkip2(t, 1, "testWithCallerSkip1", 38)
	})
	t.Run("2", func(t *testing.T) {
		testWithCallerSkip2(t, 2, "testWithCallerSkip2", 42)
	})
}

func TestWithCaller(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	want := log.Root()
	logger := mock_log.NewMockLogger(ctrl)
	logger.EXPECT().New(matchers.Slice{
		"funcName", thisPkg + ".TestWithCaller",
		"funcFile", thisFile,
		"funcLine", 67, // keep this in sync with WithCaller() call below
	}).Return(want)
	if got := WithCaller(logger); !reflect.DeepEqual(got, want) {
		t.Errorf("WithCallerSkip() = %v, want %v", got, want)
	}
}

func TestGetLogger(t *testing.T) {
	oldHandler := GetLogInstance().GetHandler()
	defer GetLogInstance().SetHandler(oldHandler)
	ctrl := gomock.NewController(t)
	handler := mock_log.NewMockHandler(ctrl)
	handler.EXPECT().Log(matchers.Struct{
		"Msg": "omg",
		"Ctx": matchers.Slice{
			"port", "", // added by the singleton instance
			"ip", "", // added by the singleton instance
			"funcName", thisPkg + ".TestGetLogger",
			"funcFile", thisFile,
			"funcLine", 88, // keep this in sync with Debug() call below
		},
	})
	GetLogInstance().SetHandler(handler)
	GetLogger().Debug("omg")
}
