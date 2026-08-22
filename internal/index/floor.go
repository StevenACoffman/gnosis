package index

// FloorBreached reports whether a rebuild is about to lose most of the corpus.
//
// Requires: previous and current are document counts; fraction is the share of
// previous below which a rebuild is treated as an accident.
// Ensures: false when previous is zero, when fraction is outside (0, 1], or when
// current is at or above the floor. Pure — it is arithmetic over three numbers, so
// the decision can be argued with in a test rather than reproduced with a corpus.
//
// **This is the one place the regenerable-cache argument turns around.** Everywhere
// else in this codebase the index being derived is what makes it safe to destroy;
// here it is what makes destroying it unnoticeable. A wrong `--bundle`, a partial
// clone, a working tree with `c/` unstaged, or a walk that stopped early all produce
// the same thing — a rebuild that does exactly what it was told and writes an index
// describing almost nothing, over the only artifact that would have shown what was
// there a moment ago.
//
// **A zero previous never breaches**, and that case is why this is a function rather
// than an inline comparison. A fresh bundle has no prior count, so a floor that fired
// on the first rebuild would make `init` followed by `index rebuild` fail on an empty
// corpus — the check would break the ordinary path in order to protect the rare one.
//
// An out-of-range fraction also never breaches. A misconfigured floor must not block
// a rebuild: the value comes from `standards/`, the loader already rejects a bad one,
// and if a bad one reaches here the conservative direction is to let work proceed
// rather than to wedge the corpus on a threshold nobody meant to set.
func FloorBreached(previous, current int, fraction float64) bool {
	if previous <= 0 || fraction <= 0 || fraction > 1 {
		return false
	}
	return float64(current) < float64(previous)*fraction
}
