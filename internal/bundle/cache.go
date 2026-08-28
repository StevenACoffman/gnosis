package bundle

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/StevenACoffman/skillet/atomicfile"
	"github.com/StevenACoffman/skillet/errs"
)

// cacheDir is the response cache, inside stateDir.
//
// Content-addressed like tier 0 and gitignored unlike it, and the asymmetry is
// the point rather than an oversight. §10.7.4's rule decides it: **decisions are
// committed, observations are cached.** An archived source is what a quotation is
// checked against — it has to travel, or a colleague cannot verify the claim. A
// model's reply is an observation somebody made at a moment, and two users at one
// commit holding different caches are both right, because one of them asked.
//
// What this gives up is that a colleague re-running ingest pays for the model
// calls again. That is the correct trade: committing replies would put unreviewed
// model output in the repository, which is precisely what quarantine exists to
// prevent one directory over.
const cacheDir = "cache"

// CachedReply is one stored answer.
//
// There is no timestamp, for §4.3.1's reason arriving a third time: the key is
// content-addressed, so a re-run over unchanged inputs finds the same entry and
// writes nothing. A timestamp would make the cache grow with asking rather than
// with knowledge, and nothing reads the history.
type CachedReply struct {
	// Key is the §6.1 cache key, stored inside the record as well as being its
	// filename, so a file copied out of place still says what it answers.
	Key string `json:"key"`

	// URI is the source the prompt was about, for a person reading the cache.
	URI string `json:"uri"`

	// Model and ModelVersion are what answered. They are already in the key; they
	// are repeated here because a key is opaque and a reader deciding whether to
	// trust a cached answer needs to see them.
	Model        string `json:"model"`
	ModelVersion string `json:"model_version"`

	// Reply is the agent's raw response, exactly as received. Not the parsed form:
	// re-parsing on read means a change to the parser applies to cached replies
	// too, where storing the parse would freeze yesterday's reading of them.
	Reply string `json:"reply"`
}

// CachePath is where a key's reply lives, relative to the bundle root.
func CachePath(key string) string {
	return filepath.ToSlash(filepath.Join(stateDir, cacheDir, key[:2], key+".json"))
}

// LoadCached reads a stored reply.
//
// Requires: key is a §6.1 cache key.
// Ensures: reports false with no error when nothing is cached, because a miss is
// the ordinary state of a first run and not a condition to handle. A cached entry
// whose stored key disagrees with its filename is an error rather than a miss:
// that is a corrupted or hand-edited cache, and treating it as absent would
// silently re-ask instead of saying so.
func LoadCached(bundleDir, key string) (CachedReply, bool, error) {
	const op = "bundle.LoadCached"

	data, err := os.ReadFile(filepath.Join(bundleDir, filepath.FromSlash(CachePath(key))))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return CachedReply{}, false, nil
		}
		return CachedReply{}, false, &errs.Error{Op: op, Err: err}
	}

	var out CachedReply
	if uErr := json.Unmarshal(data, &out); uErr != nil {
		return CachedReply{}, false, &errs.Error{Code: errs.EINVALID, Op: op, Err: uErr}
	}
	if out.Key != key {
		return CachedReply{}, false, &errs.Error{
			Code:    errs.EINVALID,
			Message: op + ": " + CachePath(key) + " holds a reply for key " + out.Key,
		}
	}
	return out, true, nil
}

// StoreCached writes a reply under its key.
//
// Requires: entry.Key is the key the reply answers.
// Ensures: written atomically, so an interrupted admit leaves no half-record that
// the next run would read as an answer. Overwrites deliberately: unlike a tier-0
// record, whose path is the hash of its own content, this path is the hash of the
// *question* — so a second answer to one question is a legitimate replacement
// rather than evidence of tampering.
func (w *Writer) StoreCached(entry *CachedReply) error {
	const op = "bundle.Writer.StoreCached"

	if err := w.held(op); err != nil {
		return err
	}
	if entry.Key == "" {
		return &errs.Error{Code: errs.EINVALID, Message: op + ": entry has no key"}
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return &errs.Error{Op: op, Err: err}
	}
	full := filepath.Join(w.dir, filepath.FromSlash(CachePath(entry.Key)))
	if mkErr := os.MkdirAll(filepath.Dir(full), 0o750); mkErr != nil {
		return &errs.Error{Op: op, Err: mkErr}
	}
	if wErr := atomicfile.WriteFile(full, append(data, '\n'), 0o640); wErr != nil {
		return &errs.Error{Op: op, Err: wErr}
	}
	return nil
}
