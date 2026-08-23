package bundle

import (
	"github.com/StevenACoffman/gnosis/internal/gate"
	"github.com/StevenACoffman/skillet/errs"
)

// Waiting is one quarantined document and what the gate makes of it.
type Waiting struct {
	// Path is the document's destination in the bundle, relative to its root.
	Path string `json:"path"`

	// Decision is the gate's conclusion. It is the field that makes a review
	// queue useful rather than merely present: a list of paths says what is
	// waiting and not why any of it is stuck, which is the actual question.
	Decision gate.Decision `json:"decision"`

	// Failed and Unchecked name the signals behind the decision, so a reader
	// deciding what to do next need not run a second command per entry.
	Failed    []gate.Signal `json:"failed,omitempty"`
	Unchecked []gate.Signal `json:"unchecked,omitempty"`
}

// Review reports every quarantined document and the gate's verdict on it.
//
// Requires: bundleDir names a bundle. An absent quarantine directory is not an
// error — a corpus with nothing waiting is the ordinary case and returns an empty
// slice.
// Ensures: sorted by path, so two runs are comparable and a review queue does not
// reorder itself under a reader. No writes: this holds no lock and takes no
// coordinator, because §4.6 requires that a reader never depend on the writer.
//
// The corpus is gathered once and reused across candidates. That is not only for
// speed: evaluating each entry against a separately-read corpus would let two
// entries be judged against different states of the bundle, and a queue whose rows
// disagree about what is already in the corpus is worse than a slow one.
//
// Every entry runs the gate, which runs the planted-defect self-test — so a queue
// of n documents runs it n times. Tier 1 is a review queue and does not reach the
// scale where that matters; if it ever does, the self-test hoists out of the loop,
// and the reason it is inside is that Evaluate is the only entry point that
// guarantees it ran at all.
func Review(bundleDir string) ([]Waiting, error) {
	const op = "bundle.Review"

	paths, err := Quarantined(bundleDir)
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}
	if len(paths) == 0 {
		return []Waiting{}, nil
	}

	c := &Coordinator{Dir: bundleDir}
	corpus, limits, err := c.gateInputs()
	if err != nil {
		return nil, &errs.Error{Op: op, Err: err}
	}

	out := make([]Waiting, 0, len(paths))
	for _, rel := range paths {
		w, wErr := c.waiting(op, rel, corpus, limits)
		if wErr != nil {
			return nil, wErr
		}
		out = append(out, w)
	}
	return out, nil
}

// waiting evaluates one quarantined document.
func (c *Coordinator) waiting(
	op, rel string, corpus *gate.Corpus, limits gate.Limits,
) (Waiting, error) {
	after, err := ReadQuarantined(c.Dir, rel)
	if err != nil {
		// The listing found it and the read did not. That is a filesystem race or
		// a failing disk, not a verdict, and reporting it as one would put a
		// hardware fault in a review queue as though somebody could edit it away.
		return Waiting{}, &errs.Error{Op: op, Err: err}
	}
	before, err := readIfPresent(op, quarantineTargetPath(c.Dir, rel))
	if err != nil {
		return Waiting{}, err
	}

	report := gate.Evaluate(c.candidate(rel, before, after), corpus, limits)
	failed, unchecked := report.Withheld()
	return Waiting{
		Path: rel, Decision: report.Decide(), Failed: failed, Unchecked: unchecked,
	}, nil
}
