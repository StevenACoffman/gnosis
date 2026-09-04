package bundle

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
	"github.com/StevenACoffman/skillet/finding"
)

// coverageFile records what earlier critiques of each claim looked at, inside stateDir
// and therefore gitignored.
//
// **The file's name is §10.5's and the Go type's is not.** That section calls this the
// coverage record and names the path, so the path is what it says; `Critique` is the
// type, because `coverage` in this codebase already means §17.3.1's lint check — a claim
// asserting more strongly than its evidence supports. Two unrelated things under one
// name is what a reader disambiguates at every call site, which is the rule that made
// §10.6.1's tier into `Authority`.
const coverageFile = "coverage.jsonl"

// Critique is one cold critic's account of what it looked at (§10.5).
//
// **It records angles and never verdicts.** That is what makes feeding it to a later
// critic safe: §10.5's argument is that a coverage record says what was *looked at*,
// never what was *concluded*, so a fresh critic reading it is steered toward unexamined
// ground rather than toward an answer. A findings field here would turn the mechanism
// into the contamination it exists to be the opposite of.
type Critique struct {
	// ClaimRef is the claim, as gnosis.ClaimRef writes it.
	ClaimRef string `json:"claim_ref"`

	// Key is the prompt this answered, so a row can be traced to the question.
	Key string `json:"key"`

	// Model is what answered. Two models covering different angles is the case this
	// ledger is most useful in, and a row that did not say which would flatten it.
	Model string `json:"model"`

	// At is when the verdict was filed. This file is *about* the sequence of
	// critiques, so a timestamp is content rather than an accident of when somebody
	// ran a sweep — `checked.jsonl`'s argument.
	At time.Time `json:"at"`

	Examined []string `json:"examined"`

	// NotExamined are the aspects this critic declined, each with the reason that
	// says whether a better excerpt would close the gap (§16.1's family type).
	//
	// **It reads a bare string as well as an object**, which is what UnmarshalJSON
	// below is for: rows written before critiques carried a reason are still steering
	// data, and turning a format change into a corruption error would fail the load of
	// the one file whose whole purpose is being matched against later.
	NotExamined []finding.Unexamined `json:"not_examined,omitempty"`
}

// UnmarshalJSON reads a critique, accepting the shape written before an unexamined
// aspect carried a reason.
//
// Requires: data is one ledger line.
// Ensures: a row whose `not_examined` holds bare strings decodes with each string as an
// aspect and a reason saying what it is. Pure.
//
// **This is "define errors out of existence" rather than a kindness.** The alternative
// is a corruption error on a file that exists to be matched against later, for a format
// nobody has had time to depend on — and the corruption path is `LoadCritiques`'s
// *correct* behaviour for a line nobody can read, which this makes readable instead.
//
// The filled-in reason says what it is rather than inventing one. A row that claimed
// "the excerpt does not include it" would be putting words in an earlier critic's mouth,
// which is the fabrication this whole block exists to prevent.
func (c *Critique) UnmarshalJSON(data []byte) error {
	const op = "bundle.Critique.UnmarshalJSON"

	// An alias, so unmarshalling into it does not call this method again.
	type plain Critique
	var row struct {
		plain
		NotExamined []json.RawMessage `json:"not_examined"`
	}
	if err := json.Unmarshal(data, &row); err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	*c = Critique(row.plain)
	c.NotExamined = nil

	for _, raw := range row.NotExamined {
		var aspect string
		if err := json.Unmarshal(raw, &aspect); err == nil {
			c.NotExamined = append(c.NotExamined, finding.Unexamined{
				Aspect: aspect,
				Reason: "recorded before critiques carried a reason",
			})
			continue
		}
		var gap finding.Unexamined
		if err := json.Unmarshal(raw, &gap); err != nil {
			return &errs.Error{Op: op, Err: err}
		}
		c.NotExamined = append(c.NotExamined, gap)
	}
	return nil
}

// LoadCritiques reads what this user's critics have covered.
//
// Requires: bundleDir is a path, which need not exist.
// Ensures: the rows in the order they were written, which is chronological; empty when
// the file is absent. A corrupt line is an error naming the line, not a silently
// shorter history — the same rule `LoadChecks` follows, and for the same reason: a
// ledger that quietly drops what it cannot read would steer the next critic with half
// the record.
func LoadCritiques(bundleDir string) ([]Critique, error) {
	const op = "bundle.LoadCritiques"

	f, err := os.Open(filepath.Join(bundleDir, stateDir, coverageFile))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = f.Close() }()

	var out []Critique
	scanner := bufio.NewScanner(f)
	line := 0
	for scanner.Scan() {
		line++
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		var c Critique
		if uErr := json.Unmarshal(scanner.Bytes(), &c); uErr != nil {
			return nil, corrupt(op, coverageFile, "line "+strconv.Itoa(line)+
				" is not a JSON object: "+uErr.Error())
		}
		out = append(out, c)
	}
	if sErr := scanner.Err(); sErr != nil {
		return nil, &errs.Error{Op: op, Err: sErr}
	}
	return out, nil
}

// RecordCritique appends one critic's coverage.
//
// Requires: the writer holds the lock; c names a claim and carries what was examined.
// Ensures: one line appended, and the file created when it is absent. The row is
// written as it is given — nothing here merges, deduplicates, or reconciles, because
// what this file holds is a sequence of accounts rather than a current state.
//
// **Appended rather than upserted, and the contrast with its two siblings is the
// reason.** `RecordChecks` and `RecordRetrievals` both rewrite a keyed row because
// "nothing consumes the sequence" — their readers want the latest value. Here the
// sequence is exactly what is consumed: §10.5 steers a later critic away from
// *exhausted* angles, which is the union across critiques, so collapsing to one row per
// claim would discard what an earlier critic covered and re-ask the questions it
// answered.
//
// 0o600 for `checked.jsonl`'s reason: this names what one person asked a model and
// when, in a gitignored directory, so no group has cause to read it.
func (w *Writer) RecordCritique(c *Critique) error {
	const op = "bundle.Writer.RecordCritique"

	if err := w.held(op); err != nil {
		return err
	}
	line, err := json.Marshal(c)
	if err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	dir := filepath.Join(w.dir, stateDir)
	if mkErr := os.MkdirAll(dir, 0o750); mkErr != nil {
		return &errs.Error{Op: op, Err: mkErr}
	}
	f, err := os.OpenFile(filepath.Join(dir, coverageFile),
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

// CoveredAngles is what earlier critiques of one claim examined, and what they declared
// they had not.
//
// Requires: critiques came from LoadCritiques; ref names the claim.
// Ensures: each list deduplicated case-insensitively and in first-written order; an
// angle appears in `notExamined` only when **no** critique reported examining it. Pure.
//
// **The subtraction is the point.** An angle one critic declined and a later one covered
// is finished, and suggesting it again would steer the next critic back onto exhausted
// ground — the opposite of what §10.5 feeds this record forward for.
//
// First-written order rather than sorted: it is chronological, it is stable across runs,
// and stability is not optional here — the lists go into the prompt, and the prompt's
// hash is the cache key.
func CoveredAngles(
	critiques []Critique, ref string,
) (examined []string, notExamined []finding.Unexamined) {
	seenExamined := map[string]bool{}
	for i := range critiques {
		if critiques[i].ClaimRef != ref {
			continue
		}
		examined = appendAngle(examined, seenExamined, critiques[i].Examined)
	}
	seenNot := map[string]bool{}
	for i := range critiques {
		if critiques[i].ClaimRef != ref {
			continue
		}
		for _, gap := range critiques[i].NotExamined {
			folded := gnosis.Surface(gap.Aspect).Fold()
			if folded == "" || seenExamined[folded] || seenNot[folded] {
				continue
			}
			seenNot[folded] = true
			notExamined = append(notExamined, gap)
		}
	}
	return examined, notExamined
}

// appendAngle adds each angle not already present, comparing case-insensitively.
//
// Two critics writing "the source's methodology" and "The source's methodology" have
// covered one angle, and a prompt listing both would spend a bullet saying so twice.
func appendAngle(out []string, seen map[string]bool, angles []string) []string {
	for _, angle := range angles {
		folded := gnosis.Surface(angle).Fold()
		if folded == "" || seen[folded] {
			continue
		}
		seen[folded] = true
		out = append(out, angle)
	}
	return out
}
