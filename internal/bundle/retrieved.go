package bundle

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/errs"
)

// retrievedFile records which claims this user's searches have returned, inside stateDir
// and therefore gitignored.
//
// **Per-user for §12.2's reason, which is the same one `checked.jsonl` gives** (§4.3.1):
// two colleagues at one commit have retrieved different things and are both right. It is
// derived state and never committed.
//
// **What it measures is reach, not reliance**, and that limit is the honest one rather
// than a shortcoming to fix later. A result returned and ignored counts here. §12.2
// weighed four stronger meanings and refused each: citation cannot be had because a link
// carries no claim target, and adjudication and verification are about truth rather than
// consultation.
const retrievedFile = "retrieved.jsonl"

// Retrieval is what one claim's search history amounts to.
type Retrieval struct {
	ClaimID string `json:"claim_id"`

	// Returns counts the times a search returned this claim.
	//
	// **A lower bound.** Recording is best-effort and takes no lock (see
	// RecordRetrievals), so two concurrent searches can lose one's update. That is
	// stated rather than engineered away because the number is never compared against
	// a target — §12.2 forbids a fraction, and a count of returns is read as "this was
	// reached at least this often".
	Returns int `json:"returns"`

	// LastAt is the most recent moment a search returned it. This file is *about*
	// time, so a timestamp here is the content rather than an accident of when
	// somebody ran a sweep — the same argument `checked.jsonl` makes.
	LastAt time.Time `json:"last_at"`
}

// LoadRetrievals reads what this user's searches have returned.
//
// Requires: bundleDir is a path, which need not exist.
// Ensures: an empty map when the file is absent, keyed by claim id. A corrupt line is an
// error naming the line, not a silently shorter history.
func LoadRetrievals(bundleDir string) (map[string]Retrieval, error) {
	const op = "bundle.LoadRetrievals"

	f, err := os.Open(filepath.Join(bundleDir, stateDir, retrievedFile))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]Retrieval{}, nil
		}
		return nil, &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = f.Close() }()

	out := map[string]Retrieval{}
	scanner := bufio.NewScanner(f)
	line := 0
	for scanner.Scan() {
		line++
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		var r Retrieval
		if uErr := json.Unmarshal(scanner.Bytes(), &r); uErr != nil {
			return nil, corrupt(op, retrievedFile, "line "+strconv.Itoa(line)+
				" is not a JSON object: "+uErr.Error())
		}
		out[r.ClaimID] = r
	}
	if sErr := scanner.Err(); sErr != nil {
		return nil, &errs.Error{Op: op, Err: sErr}
	}
	return out, nil
}

// RecordRetrievals notes that a search returned these claims now.
//
// Requires: at is the moment of the search; claimIDs may be empty.
// Ensures: each claim's count rises by one and its timestamp becomes at. The file is
// rewritten atomically in a stable order, so two runs over one state produce identical
// bytes. Upsert rather than append, for the reason `RecordChecks` gives: nothing consumes
// the *sequence*, and an append-only log here would grow without bound in a file whose
// only reader wants the latest.
//
// **A free function rather than a Writer method, and that is the whole design decision.**
// Every other record in this system goes through the write coordinator's lock. This one
// must not: `search` is a read command, and putting a retrieval behind the writer's lock
// would serialise reads behind every writer and make searching a corpus wait on somebody
// else's ingest. §4.6's coordinator owns the *bundle*; this file is derived, per-user,
// and gitignored, so nothing it can lose is a fact about the corpus.
//
// **What that costs is stated in Retrieval.Returns**: two concurrent searches can lose an
// update. The failure direction is a claim looking *less* retrieved than it was, which is
// why the report says "not observed retrieved" rather than "never retrieved". A signal
// that erred the other way — claiming reach it did not observe — would be the one worth
// engineering against.
func RecordRetrievals(bundleDir string, at time.Time, claimIDs []string) error {
	const op = "bundle.RecordRetrievals"

	if len(claimIDs) == 0 {
		return nil
	}
	existing, err := LoadRetrievals(bundleDir)
	if err != nil {
		return err
	}
	for _, id := range claimIDs {
		if id == "" {
			continue
		}
		r := existing[id]
		r.ClaimID = id
		r.Returns++
		r.LastAt = at
		existing[id] = r
	}

	body, err := renderRetrievals(op, existing)
	if err != nil {
		return err
	}
	dir := filepath.Join(bundleDir, stateDir)
	if mkErr := os.MkdirAll(dir, 0o750); mkErr != nil {
		return &errs.Error{Op: op, Err: mkErr}
	}
	// 0o600 for `checked.jsonl`'s reason: this names what one person searched for and
	// when, in a gitignored directory, so no group has cause to read it.
	if wErr := atomicfile.WriteFile(
		filepath.Join(dir, retrievedFile), body, 0o600,
	); wErr != nil {
		return &errs.Error{Op: op, Err: wErr}
	}
	return nil
}

// renderRetrievals writes the file, sorted by claim id so the bytes are stable.
func renderRetrievals(op string, rows map[string]Retrieval) ([]byte, error) {
	keys := make([]string, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var out []byte
	for _, k := range keys {
		line, err := json.Marshal(rows[k])
		if err != nil {
			return nil, &errs.Error{Op: op, Err: err}
		}
		out = append(out, line...)
		out = append(out, '\n')
	}
	return out, nil
}
