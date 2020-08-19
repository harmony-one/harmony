package herrors

type multiErr []error

// Error indicates multiErr implements error interface
func (errs multiErr) Error() string {
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

func Join(errs ...error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}
	return multiErr(errs)
}
