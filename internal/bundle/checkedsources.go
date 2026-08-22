package bundle

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/errs"
)

// checkedFile records when this user last verified each source, inside stateDir
// and therefore gitignored.
//
// **This is the one documented exception to §4.5**, and §4.3.1 explains why it is
// per-user rather than committed: that upstream *changed* is a fact about the
// corpus, produces new bytes and a new record, and must travel. That *I looked and
// nothing had changed* is an observation made at a moment, and two colleagues who
// report different freshness at the same commit are both right, because one of
// them looked.
//
// It is also what makes omitting the fetch record's timestamp affordable (§4.3.1).
// Without it a re-fetch of unchanged bytes leaves no trace at all, and §14.3 cannot
// tell "checked, unchanged" from "never checked" — which is the collapse the
// four-state vocabulary exists to prevent.
const checkedFile = "checked.jsonl"

// Check is one observation: this user looked at this version of this source at
// this moment and it had not changed.
type Check struct {
	URI string `json:"uri"`

	// SourceSHA256 is the version that was current when the check happened. A
	// check is about a *version*, not a URI: learning that v1 is unchanged says
	// nothing once v2 exists, and keying on the URI alone would let an old
	// observation vouch for new bytes.
	SourceSHA256 string `json:"source_sha256"`

	// At is when. Unlike a fetch record this file is *about* time, so a timestamp
	// here is the content rather than an accident of when somebody ran a sweep.
	At time.Time `json:"at"`
}

// key is the identity of an observation: a source version, not a source.
func (c Check) key() string { return c.URI + "\x00" + c.SourceSHA256 }

// LoadChecks reads this user's check record.
//
// Requires: nothing; a bundle nobody has checked is not an error.
// Ensures: the latest observation per (uri, source version), keyed for lookup.
// Empty rather than nil for an absent file, so a caller need not distinguish
// "never checked anything" from "no result". A malformed line is an error: this
// file decides whether a claim reads as verified, and silently dropping a line
// would make a source report as never-checked when the record exists.
func LoadChecks(bundleDir string) (map[string]Check, error) {
	const op = "bundle.LoadChecks"

	f, err := os.Open(filepath.Join(bundleDir, stateDir, checkedFile))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]Check{}, nil
		}
		return nil, &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = f.Close() }()

	out := map[string]Check{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		var c Check
		if uErr := json.Unmarshal(scanner.Bytes(), &c); uErr != nil {
			return nil, &errs.Error{Code: errs.EINVALID, Op: op, Err: uErr}
		}
		out[c.key()] = c
	}
	if sErr := scanner.Err(); sErr != nil {
		return nil, &errs.Error{Op: op, Err: sErr}
	}
	return out, nil
}

// RecordChecks notes that this user looked at these source versions now.
//
// Requires: the writer lock is held; at is the moment of the check.
// Ensures: each observation replaces any earlier one for the same source version,
// and the file is rewritten atomically in a stable order so two runs over one
// state produce identical bytes.
//
// **Upsert rather than append**, unlike every other record in this system. §4.3.1
// says nothing consumes the sequence of checks — §14.3 needs only the latest — so
// an append-only log here would be exactly the unbounded growth that section
// refused for tier 0, in a file with even less reason to keep history. A weekly
// sweep over 500 sources appends 26,000 lines a year that no reader ever reads.
//
// Rewriting the whole file is affordable by construction: one line per distinct
// source version, so a 500-source corpus writes about 60 KB. That is cheaper than
// the compaction pass an append-and-supersede design would eventually need, and it
// removes the last-line-wins subtlety a reader would otherwise have to know about.
func RecordChecks(bundleDir string, at time.Time, sources []Check) error {
	const op = "bundle.RecordChecks"

	if len(sources) == 0 {
		return nil
	}
	existing, err := LoadChecks(bundleDir)
	if err != nil {
		return err
	}
	for _, s := range sources {
		s.At = at
		existing[s.key()] = s
	}

	body, err := renderChecks(op, existing)
	if err != nil {
		return err
	}
	dir := filepath.Join(bundleDir, stateDir)
	if mkErr := os.MkdirAll(dir, 0o750); mkErr != nil {
		return &errs.Error{Op: op, Err: mkErr}
	}
	// 0o600: this names what one user looked at and when, in a gitignored
	// directory, so there is no group with a reason to read it.
	if wErr := atomicfile.WriteFile(filepath.Join(dir, checkedFile), body, 0o600); wErr != nil {
		return &errs.Error{Op: op, Err: wErr}
	}
	return nil
}

// renderChecks writes the file, sorted by key so the bytes are stable.
func renderChecks(op string, checks map[string]Check) ([]byte, error) {
	keys := make([]string, 0, len(checks))
	for k := range checks {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var out []byte
	for _, k := range keys {
		line, err := json.Marshal(checks[k])
		if err != nil {
			return nil, &errs.Error{Op: op, Err: err}
		}
		out = append(out, line...)
		out = append(out, '\n')
	}
	return out, nil
}
