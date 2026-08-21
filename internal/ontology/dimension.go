package ontology

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
