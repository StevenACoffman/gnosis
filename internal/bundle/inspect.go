package bundle

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/StevenACoffman/gnosis/internal/index"
	"github.com/StevenACoffman/gnosis/internal/lint"
	"github.com/StevenACoffman/gnosis/internal/ontology"
	"github.com/StevenACoffman/skillet/errs"
)

// Inspect gathers the state of the apparatus around a bundle.
//
// Requires: dir names a directory, or nothing — an absent bundle is inspectable
// and produces the findings that say so.
// Ensures: no error is returned for anything that is a finding. A missing
// vocabulary, an absent index, and an unparsable ontology are all reported
// through the returned Environment, because a diagnostic command that fails
// instead of diagnosing is useless exactly when it is needed. An error is
// returned only when the filesystem itself misbehaves.
func Inspect(ctx context.Context, dir string) (lint.Environment, error) {
	const op = "bundle.Inspect"

	env := lint.Environment{Bundle: dir, SchemaVersion: index.SchemaVersion()}

	env.OntologyPresent, env.OntologyError, env.Types = inspectOntology(dir)
	env.Archive, env.StandardsError = inspectArchive(dir)
	env.IndexDocPresent = exists(filepath.Join(dir, "index.md"))
	env.StateIgnored = stateIgnored(dir)

	docs, err := Load(os.DirFS(dir))
	if err != nil {
		return env, &errs.Error{Op: op, Err: err}
	}
	env.Documents = len(docs)

	idx, err := LoadIndex(ctx, dir)
	if err != nil {
		return env, &errs.Error{Op: op, Err: err}
	}
	env.IndexPresent = idx.Present
	env.IndexedRows = len(idx.Rows)
	if idx.Present {
		env.IndexVersion, env.SchemaMissing, env.SchemaUnexpected, err = inspectIndex(ctx, dir)
		if err != nil {
			return env, &errs.Error{Op: op, Err: err}
		}
	}
	return env, nil
}

// inspectOntology reads and validates the vocabulary, reporting a load failure
// as text rather than an error.
func inspectOntology(dir string) (present bool, loadErr string, types int) {
	raw, err := os.ReadFile(filepath.Join(dir, ontology.FileName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, "", 0
		}
		return true, err.Error(), 0
	}
	o, err := ontology.Load(raw)
	if err != nil {
		return true, err.Error(), 0
	}
	return true, "", len(o.Types)
}

// stateIgnored reports whether .gitignore excludes the derived state directory.
//
// The test is a substring rather than a parse of gitignore's pattern language:
// what matters is whether somebody has addressed the question, and a pattern
// mentioning the directory is evidence they have. Reimplementing gitignore
// matching to answer a hygiene warning would cost more than the warning is
// worth.
func stateIgnored(dir string) bool {
	raw, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		return false
	}
	return bytes.Contains(raw, []byte(stateDir))
}

// inspectIndex reads the version the database reports and how its schema differs
// from what the migrations describe. Both come from one open, because opening
// twice would let the two answers describe different moments.
func inspectIndex(
	ctx context.Context,
	dir string,
) (version int, missing, unexpected []string, err error) {
	const op = "bundle.inspectIndex"

	db, err := index.Open(ctx, IndexPath(dir))
	if err != nil {
		return 0, nil, nil, &errs.Error{Op: op, Err: err}
	}
	defer func() { _ = db.Close() }()

	version, err = db.Version(ctx)
	if err != nil {
		return 0, nil, nil, &errs.Error{Op: op, Err: err}
	}
	shape, err := db.CheckShape(ctx)
	if err != nil {
		return 0, nil, nil, &errs.Error{Op: op, Err: err}
	}
	return version, shape.Missing, shape.Unexpected, nil
}

// exists reports whether a path is present.
// inspectArchive measures tier 0 against its declared budget, and reports why it
// could not.
//
// A failure does not stop the other checks — `doctor` describes the whole
// apparatus and one broken part must not hide the rest — but it is **returned
// rather than swallowed**. The first version dropped it and returned a zero size,
// which reads as "no budget declared" and produced a clean bill of health for a
// corpus whose standards file does not parse. That is the one output this command
// must never produce.
func inspectArchive(dir string) (lint.ArchiveSize, string) {
	std, err := LoadArchiveStandards(dir)
	if err != nil {
		return lint.ArchiveSize{}, err.Error()
	}
	size, err := MeasureArchive(dir, std.CorpusBudget.Value, std.CorpusWarnFraction.Value)
	if err != nil {
		return lint.ArchiveSize{}, err.Error()
	}
	return size, ""
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
