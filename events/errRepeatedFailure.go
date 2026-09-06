package events

import "errors"

// ErrRepeatedFailure marks a chain that stopped on a failure the trade already
// carries as its newest row: tradelog.SaveError found the same "Insufficient
// funds" / exchange error there and wrote nothing. The tick still failed —
// nothing happened on it — so Run keeps the backoff, which live is what stops a
// blocked trade from asking the exchange for its balances on every price
// print. It is NOT logged again: the first failure wrote the row and the error
// line, and every later tick of a blocked trade repeats it word for word — on a
// deep ladder out of funds that was two identical lines per print, thousands a
// minute, in the backtest and live alike.
var ErrRepeatedFailure = errors.New("failure already on the trade")

// Repeated wraps err so errors.Is(err, ErrRepeatedFailure) holds while the
// message stays byte for byte the gate's or the exchange's own text, which the
// ERROR rows and the run statistics key on.
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
