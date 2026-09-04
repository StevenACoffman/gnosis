package bundle

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// missFile records every prompt this user's gnosis emitted, inside stateDir and
// therefore gitignored (§6.4).
//
// **It is a tracer**: instrumentation added so that something which otherwise leaves no
// trace begins leaving one. A miss is a non-event — nothing happened,
// deterministically, and then a model was asked — and without the log the determinism
// claim is untestable, because a claim about how rarely the model is consulted needs
// the consultations counted.
//
// Per-user and derived, like `checked.jsonl` and `retrieved.jsonl`: two colleagues at
// one commit have asked models different things and are both right.
const missFile = "miss.jsonl"

// Miss is one prompt emission, in §6.4's shape.
//
// **A file rather than a table, and §12's own rule says which.** The rows are
// append-only, per-user, read whole by one aggregator, and never joined; that is a log,
// not a relation. Putting it in SQLite would add a migration, a projection and an
// `index rebuild` story to state that must not survive a rebuild anyway — §10.7.4 puts
// observations in the cache.
type Miss struct {
	// Op is the operation that asked, in §6.4's vocabulary: `extract`, `accrete`,
	// `synthesize`, `critique`, and `conflict-adjudicate` when that exists.
	Op string `json:"op"`

	// Key is the §6.1 cache key of the prompt this row is about.
	//
	// **It is what makes a question countable, and running the binary is what showed
	// the need.** Two `gnosis ingest` runs over one unanswered source write the same
	// prompt twice — the reply is not cached, so nothing is skipped — and the log then
	// held two rows for one question. The rows are right: two emissions happened. The
	// *count* was wrong, and it was wrong in the direction that flatters nobody and
	// misleads everybody, because a reader of the report is asking how often a model
	// was consulted.
	Key string `json:"key"`

	// Reason is why a model was asked rather than the question answered locally.
	Reason gnosis.MissReason `json:"reason"`

	// ChecksRun and ChecksFired are the deterministic predicates that were tried and
	// the ones that decided something.
	//
	// **Both empty is honest for an emitter that ran no checks**, and fabricating a
	// list would make the report claim a predicate had been tried. §6.4's example is
	// a conflict adjudication, where the pair is the whole finding: checks run,
	// nothing fired, so a predicate is missing.
	ChecksRun   []string `json:"checks_run,omitempty"`
	ChecksFired []string `json:"checks_fired,omitempty"`

	// Candidate is what the prompt was about — a source URI or a claim reference.
	// Other is the second artifact where the question concerned a pair.
	Candidate string `json:"candidate"`
	Other     string `json:"other,omitempty"`

	// At is when. This file is *about* the sequence of consultations, so the
	// timestamp is content rather than an accident of when somebody ran a sweep —
	// `checked.jsonl`'s argument.
	At time.Time `json:"at"`
}

// MissGroup is what one reason accounts for, as `gnosis miss report` renders it.
type MissGroup struct {
	Reason gnosis.MissReason `json:"reason"`

	// Count is how many distinct questions this reason asked, by prompt key.
	//
	// **Distinct questions rather than rows**, because re-emitting an unanswered
	// prompt writes a second row and asks nothing new: the same key is the same
	// question, and a reader of this report wants to know how often a model was
	// consulted. The rows are still all there — see Emissions — because the file is a
	// log of what happened and this is the interpretation of it.
	//
	// **A count and never a rate.** §6.4.1 forbids reading this log as accuracy —
	// "a retrieval path that confidently returns the wrong concept every time has a
	// perfect miss-log record" — and §17 forbids a count presented as health. A
	// percentage here would be the most target-shaped number the corpus could
	// produce, and it improves when somebody stops asking.
	Count int `json:"count"`

	// Emissions is how many rows this reason holds, which is at least Count.
	//
	// The gap between the two is informative rather than noise: it counts the times
	// somebody re-ran a prompt nobody had answered, which is a fact about how the
	// relay is being used and not about the corpus.
	Emissions int `json:"emissions"`

	// Actionable reports whether a recurrence of this reason is a check waiting to be
	// written, which is what §6.4 says the log is for.
	Actionable bool `json:"actionable"`

	// Ops are the operations that gave this reason, sorted, so a group of a thousand
	// says which command produced it.
	Ops []string `json:"ops"`

	// ChecksRun are the predicates tried across this group, sorted and deduplicated.
	// Empty for a reason whose emitters run none.
	ChecksRun []string `json:"checks_run,omitempty"`
}

// LoadMisses reads every prompt this user has emitted.
//
// Requires: bundleDir is a path, which need not exist.
// Ensures: the rows in the order they were written, which is chronological; nil when
// the file is absent, because a corpus that has asked nothing is the ordinary state of
// a first run. A corrupt line is an error naming the line, not a silently shorter
// history — the rule every sibling ledger follows, and the one that matters most here:
// this file's only output is a count, and a count over rows that were quietly dropped
// is wrong rather than incomplete.
func LoadMisses(bundleDir string) ([]Miss, error) {
	const op = "bundle.LoadMisses"

	f, err := os.Open(filepath.Join(bundleDir, stateDir, missFile))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = f.Close() }()

	var out []Miss
	scanner := bufio.NewScanner(f)
	line := 0
	for scanner.Scan() {
		line++
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		var m Miss
		if uErr := json.Unmarshal(scanner.Bytes(), &m); uErr != nil {
			return nil, corrupt(op, missFile, "line "+strconv.Itoa(line)+
				" is not a JSON object: "+uErr.Error())
		}
		out = append(out, m)
	}
	if sErr := scanner.Err(); sErr != nil {
		return nil, &errs.Error{Op: op, Err: sErr}
	}
	return out, nil
}

// RecordMiss appends one prompt emission.
//
// Requires: the writer holds the lock; m names an operation, a reason and a candidate.
// Ensures: one line appended, and the file created when it is absent. EINVALID for a
// row with no reason: the report's whole output is a count grouped by reason, and a row
// that named none would swell whichever group it defaulted to.
//
// **A cached answer is not a miss, and the caller is where that is decided.** §6.4 logs
// a prompt *emission*; a cache hit asked nothing of a model, and a log that counted hits
// would measure how often somebody ran gnosis rather than how often the model was
// consulted — which is the one thing this file exists to measure.
//
// 0o600 for `checked.jsonl`'s reason: it names what one person asked a model and when.
func (w *Writer) RecordMiss(m *Miss) error {
	const op = "bundle.Writer.RecordMiss"

	if err := w.held(op); err != nil {
		return err
	}
	if m.Reason == gnosis.MissReasonUnset {
		return &errs.Error{
			Code: errs.EINVALID,
			Message: op + ": a miss with no reason cannot be grouped, and the group" +
				" is the report",
		}
	}
	line, err := json.Marshal(m)
	if err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	dir := filepath.Join(w.dir, stateDir)
	if mkErr := os.MkdirAll(dir, 0o750); mkErr != nil {
		return &errs.Error{Op: op, Err: mkErr}
	}
	f, err := os.OpenFile(filepath.Join(dir, missFile),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = f.Close() }()

	if _, wErr := f.Write(append(line, '\n')); wErr != nil {
		return &errs.Error{Op: op, Err: wErr}
	}
	return nil
}

// MissReport groups misses by reason (§6.4).
//
// Requires: misses came from LoadMisses.
// Ensures: one group per distinct reason, sorted so the actionable ones come first and
// then by descending count; every group names the operations behind it. Pure.
//
// **Actionable first, and that ordering is the report.** §6.4's payoff is that "a reason
// that recurs is a deterministic check waiting to be written", and that is true of one
// reason and false of the other: extraction has no deterministic alternative and never
// will, so its row is the largest and the least interesting. Sorting by count alone
// would put the line nobody can act on at the top of every run.
//
// A reason this build does not recognise is kept as its own group rather than dropped or
// folded, because a row written by a later gnosis is evidence about the corpus and a
// silently merged count is a wrong answer rather than a missing one.
func MissReport(misses []Miss) []MissGroup {
	byReason := map[gnosis.MissReason]*MissGroup{}
	ops := map[gnosis.MissReason]map[string]bool{}
	checks := map[gnosis.MissReason]map[string]bool{}
	keys := map[gnosis.MissReason]map[string]bool{}

	for i := range misses {
		m := &misses[i]
		g := byReason[m.Reason]
		if g == nil {
			g = &MissGroup{Reason: m.Reason, Actionable: m.Reason.Actionable()}
			byReason[m.Reason] = g
			ops[m.Reason] = map[string]bool{}
			checks[m.Reason] = map[string]bool{}
			keys[m.Reason] = map[string]bool{}
		}
		g.Emissions++
		if m.Op != "" {
			ops[m.Reason][m.Op] = true
		}
		for _, c := range m.ChecksRun {
			checks[m.Reason][c] = true
		}
		// A row with no key is one written before the field existed. It counts as its
		// own question rather than being dropped or merged: the emission happened, and
		// guessing which question it was is how a count becomes a wrong answer.
		keys[m.Reason][keyOrRow(m, i)] = true
	}

	out := make([]MissGroup, 0, len(byReason))
	for reason, g := range byReason {
		g.Ops = sortedKeys(ops[reason])
		g.ChecksRun = sortedKeys(checks[reason])
		g.Count = len(keys[reason])
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Actionable != out[j].Actionable {
			return out[i].Actionable
		}
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Reason < out[j].Reason
	})
	return out
}

// keyOrRow identifies the question a row is about, falling back to the row's position.
//
// A row written before `Key` existed cannot be deduplicated against anything, and the
// two wrong answers are to drop it (undercounting a consultation that happened) or to
// merge every such row into one (undercounting worse). Its index is unique and says
// what it is: one emission nobody can pair with another.
func keyOrRow(m *Miss, index int) string {
	if m.Key != "" {
		return m.Key
	}
	return "row-" + strconv.Itoa(index)
}

// sortedKeys is a set as a sorted slice, or nil when the set is empty.
//
// Nil rather than an empty slice so an `omitempty` field disappears: a report line
// carrying `checks_run: []` says a predicate list was considered and found empty, and
// for an emitter that runs no checks the honest statement is silence.
func sortedKeys(set map[string]bool) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// noteMiss records one prompt emission, reporting a failure as a note.
//
// **Best-effort, and it does not become the emission's failure.** The prompt is on disk
// and an agent can answer it; refusing the operation because a tracer could not be
// appended would trade the thing the caller asked for against the instrumentation about
// it. `Coordinator.spend` and `spendCritic` make the same call for the same reason.
//
// The direction it fails in is stated rather than left to be inferred: a lost row makes
// the corpus look as though the model was consulted **less** often than it was, which is
// the flattering direction — so the report says "at least this many" and never "only
// this many". §6.4.1's limit on what the log can support applies twice over to a log
// that can drop a line.
func (w *Writer) noteMiss(m *Miss, warn io.Writer) {
	if err := w.RecordMiss(m); err != nil && warn != nil {
		_, _ = fmt.Fprintf(warn,
			"warning: the prompt was emitted and the miss was not recorded, so "+
				"`gnosis miss report` will undercount: %v\n", err)
	}
}
