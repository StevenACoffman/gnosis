package ontology

import "strings"

// The closed set of dimensions. Adding one means adding a parser, so the set
// grows only alongside an implementation that can compare its values.
const (
	DimensionCount    Dimension = "count"
	DimensionDuration Dimension = "duration"
	DimensionBytes    Dimension = "bytes"
	DimensionRatio    Dimension = "ratio"
)

// Dimension names the parser that applies to a subject's values, and therefore
// the base unit they normalise to.
//
// An unknown dimension is a load error rather than a silent skip: a subject
// whose values cannot be parsed is a subject that will quietly never produce a
// conflict, which looks identical to a corpus that simply has no conflicts.
type Dimension string

// valid reports whether d is one of the declared dimensions.
func (d Dimension) valid() bool {
	switch d {
	case DimensionCount, DimensionDuration, DimensionBytes, DimensionRatio:
		return true
	default:
		return false
	}
}

// DimensionWritten reads the dimension a raw value's unit belongs to.
//
// Requires: raw is the span a constraint was read from, such as "400ms" or "3".
// Ensures: comma-ok. A value with no unit yields false rather than DimensionCount,
// because "3" is what *every* dimension's value looks like when the author omitted the
// unit — reading it as a count would manufacture a mismatch out of ordinary shorthand.
// Pure.
func DimensionWritten(raw string) (Dimension, bool) {
	suffix := ""
	for i := len(raw); i > 0; i-- {
		r := rune(raw[i-1])
		if r >= '0' && r <= '9' || r == '.' {
			suffix = raw[i:]
			break
		}
	}
	if suffix == "" {
		// Either no digits at all, or digits with nothing after them.
		if raw != "" && !strings.ContainsAny(raw, "0123456789") {
			return "", false
		}
	}
	return unitOf(strings.ToLower(strings.Trim(suffix, ".,;:)")))
}

// unitOf maps a written unit to the dimension it belongs to.
//
// **These are facts rather than judgements**, which is what makes a table here
// legitimate where §6.2 would refuse an invented threshold: `ms` is a duration in every
// corpus, and no bundle gets to decide otherwise. A dimension with no units — `count` —
// is absent by construction, because "3" carries no suffix to read.
//
// `%` is `ratio`'s only member and is written against the number the way a unit is, so it
// is read the same way.
//
// A switch rather than a package-level map: the set never varies at run time, and a
// global would be state where a closed list is meant.
func unitOf(unit string) (Dimension, bool) {
	switch unit {
	case "ns", "us", "µs", "ms", "s", "sec", "secs",
		"m", "min", "mins", "h", "hr", "hrs", "d":
		return DimensionDuration, true
	case "b", "kb", "mb", "gb", "tb", "kib", "mib", "gib":
		return DimensionBytes, true
	case "%":
		return DimensionRatio, true
	default:
		return "", false
	}
}
