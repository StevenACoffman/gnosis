package standards

import (
	"sort"
	"strconv"
	"strings"

	"github.com/StevenACoffman/skillet/errs"
)

// validate rejects gate thresholds outside the range in which they mean anything.
//
// As with Archive's, the checks are deliberately weak: they catch a value that
// cannot be intended, not one that is unwise. Whether three softening phrases is
// the right limit is a judgment the rationale records and a reviewer makes;
// whether a negative limit is meant is not.
//
// Requires: p is decoded.
// Ensures: EINVALID naming every problem at once, sorted, so one run surfaces the
// whole edit.
func (p *Promote) validate(op string) error {
	var bad []string
	if p.HedgingMax.Value < 0 {
		bad = append(bad, "hedging_max must not be negative")
	}
	if f := p.RebuildFloorFraction.Value; f <= 0 || f > 1 {
		bad = append(bad, "rebuild_floor_fraction must be in (0, 1]")
	}
	if len(bad) == 0 {
		return nil
	}
	sort.Strings(bad)
	return &errs.Error{Code: errs.EINVALID, Message: op + ": " + strings.Join(bad, "; ")}
}

// ComparePromote reports which gate thresholds old → cur moved in the loosening
// direction.
//
// Requires: both are loaded.
// Ensures: sorted by key, empty when nothing loosened. A sibling of CompareArchive
// rather than a merger with it, because the two files load independently and a
// bundle may have one and not the other.
//
// **The two directions here disagree, which is the argument for keeping direction
// in Go.** A higher hedging limit admits more and is a loosening; a *lower* rebuild
// floor refuses fewer rebuilds and is also a loosening. A file that declared its
// own direction would let either be inverted in the same commit that moved it.
func ComparePromote(old, cur *Promote) []Loosening {
	if old == nil || cur == nil {
		return nil
	}
	var out []Loosening

	if cur.HedgingMax.Value > old.HedgingMax.Value {
		out = append(out, Loosening{
			Key:       "hedging_max",
			From:      strconv.Itoa(old.HedgingMax.Value),
			To:        strconv.Itoa(cur.HedgingMax.Value),
			Rationale: cur.HedgingMax.Rationale,
		})
	}
	if cur.RebuildFloorFraction.Value < old.RebuildFloorFraction.Value {
		out = append(out, Loosening{
			Key:       "rebuild_floor_fraction",
			From:      formatFraction(old.RebuildFloorFraction.Value),
			To:        formatFraction(cur.RebuildFloorFraction.Value),
			Rationale: cur.RebuildFloorFraction.Rationale,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}
