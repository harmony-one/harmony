package matchers

import (
	"github.com/golang/mock/gomock"
)

func toMatcher(v interface{}) gomock.Matcher {
	if m, ok := v.(gomock.Matcher); ok {
		return m
	}
	return gomock.Eq(v)
}
