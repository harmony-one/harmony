package herrors

import "errors"

type multiError []error

// Error indicates multiError implements error interface
func (errs multiError) Error() string {
	if len(errs) == 0 {
		return ""
	}
	separator := "; "
	errStr := ""
	for _, err := range errs {
		if err == nil {
			continue
		}
		if len(errStr) != 0 {
			errStr += separator
		}
		errStr += err.Error()
	}
	return errStr
}

// multiError implements Is interface used at libexec/src/errors/wrap.go
func (errs multiError) Is(target error) bool {
	for _, err := range errs {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

// Join will join multiple errors to multiError.
// TODO: optimize slice. Merge the underlying errors if an err is a multiError itself
func Join(errs ...error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}
	return multiError(errs)
}
