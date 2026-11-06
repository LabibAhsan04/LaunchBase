package deployments

import "errors"

type transientError struct {
	err error
}

func (e *transientError) Error() string {
	if e.err == nil {
		return "transient"
	}
	return e.err.Error()
}

func (e *transientError) Unwrap() error { return e.err }

// Transient marks an error the worker may retry. Build, detect, and health
// failures are left as ordinary errors and are not retried.
func Transient(err error) error {
	if err == nil {
		return nil
	}
	return &transientError{err: err}
}

func IsTransient(err error) bool {
	var target *transientError
	return errors.As(err, &target)
}
