package ccxt

import (
	"github.com/adshao/go-binance/v2/common"
)

// apiError converts an arbitrary error coming out of CCXT (or anywhere else
// inside this adaptor) into mercury's existing `*common.APIError` shape, so
// the rest of the platform — which still types its error returns against
// the go-binance error type for legacy reasons — keeps compiling without
// touch. This is the same trick the binance adaptor's ApiError() function
// uses; centralising it here avoids duplicating the conversion.
//
// CCXT's own Go errors are plain `error` values produced by `CreateReturnError`
// inside the generated wrappers. They do not carry an HTTP code or a parsed
// response body — only the human-readable message. We surface that message in
// the `Message` field and leave `Code` at zero. Callers that key on specific
// Binance error codes (`-2010` etc.) for trade routing decisions are
// concentrated in `agora/jobs/processPendingOrder.go`; that path branches on
// the *legacy* binance adaptor codes today and will keep working under the
// binance-legacy backend. After the CCXT cutover, those branches will need to
// match on the string instead — tracked as a follow-up.
func apiError(err error) *common.APIError {
	if err == nil {
		return nil
	}
	return &common.APIError{
		Code:    0,
		Message: err.Error(),
	}
}
