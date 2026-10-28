package gnosis

import (
	"github.com/StevenACoffman/skillet/errs"
	"github.com/StevenACoffman/toerr/errors/errcode"
)

// The five classifications gnosis gives an error, as `errcode` statuses.
//
// # Why these exist rather than skillet's string codes at every site
//
// The family's division is **toerr wraps, errcode classifies** (skillet's own words), so
// a coded leaf is `errcode.New(status, message)` and a coded wrap is
// `errcode.Link(status, message, cause)`. `errcode`'s vocabulary is wider than gnosis
// needs — sixteen statuses, gRPC's canonical codes apart from OK — and two of them
// map onto one skillet code each way. Naming the five gnosis uses puts that choice in one
// place instead of at every call site, and makes the round trip below assertable.
//
// # Every one of them round-trips
//
// `errs.ErrorCode` reads an `errcode` error and returns skillet's string vocabulary,
// which is what the §8.0 envelope's `reason` tokens are built from. Each constant here is
// the status that reads back as the code gnosis meant:
//
//	CodeInvalid      → errs.EINVALID
//	CodeNotFound     → errs.ENOTFOUND
//	CodeConflict     → errs.ECONFLICT
//	CodeUnauthorized → errs.EUNAUTHORIZED
//	CodeInternal     → errs.EINTERNAL
//
// **Two are two-to-one and the arm is chosen deliberately.** `codeFromStatus` maps both
// `AlreadyExists` and `FailedPrecondition` to `ECONFLICT`, and both `Unauthenticated` and
// `PermissionDenied` to `EUNAUTHORIZED`. Picking either arm classifies identically;
// picking the first of each keeps the mapping a function in both directions, which is
// what the test asserts and what makes a reader able to predict the answer.
const (
	CodeInvalid      = errcode.StatusInvalidArgument
	CodeNotFound     = errcode.StatusNotFound
	CodeConflict     = errcode.StatusAlreadyExists
	CodeUnauthorized = errcode.StatusUnauthenticated
	CodeInternal     = errcode.StatusInternal
)

// Codes lists the five with the classification each reads back as.
//
// Requires: nothing.
// Ensures: a freshly built map, so a caller cannot mutate the table another caller sees.
// Pure.
//
// It exists for the round-trip test rather than for production code, and that is worth
// saying: a table nothing asserts against is a comment, and this mapping is the one thing
// in the migration that could be wrong everywhere at once.
func Codes() map[errcode.StatusCode]string {
	return map[errcode.StatusCode]string{
		CodeInvalid:      errs.EINVALID,
		CodeNotFound:     errs.ENOTFOUND,
		CodeConflict:     errs.ECONFLICT,
		CodeUnauthorized: errs.EUNAUTHORIZED,
		CodeInternal:     errs.EINTERNAL,
	}
}
