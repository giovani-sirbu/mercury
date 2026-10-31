package events

import "errors"

// ErrRepeatedFailure marks a chain that stopped on a failure the trade already
// carries as its newest row (tradelog.SaveError wrote nothing). The tick still
// failed, so Run keeps the backoff, but the failure is not logged again: the
// first one wrote the row and the error line.
var ErrRepeatedFailure = errors.New("failure already on the trade")

// Repeated wraps err so errors.Is(err, ErrRepeatedFailure) holds while the
// message stays the gate's or the exchange's own text, which the rows key on.
func Repeated(err error) error {
	return repeatedFailure{err: err}
}

type repeatedFailure struct {
	err error
}

func (r repeatedFailure) Error() string {
	return r.err.Error()
}

func (r repeatedFailure) Unwrap() error {
	return r.err
}

func (r repeatedFailure) Is(target error) bool {
	return target == ErrRepeatedFailure
}
