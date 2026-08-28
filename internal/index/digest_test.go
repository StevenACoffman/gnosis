package index_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/index"
)

// docRows is a small deterministic corpus, as index rows.
func docRows() []index.DocumentRow {
	return []index.DocumentRow{
		{
			ID:    "01932b7c-1f4e-7a3d-9c2b-000000000001",
			Path:  "c/a.md",
			Title: "A",
			Slug:  "a",
			Hash:  "h1",
			Bytes: 10,
		},
		{
			ID:    "01932b7c-1f4e-7a3d-9c2b-000000000002",
			Path:  "c/b.md",
			Title: "B",
			Slug:  "b",
			Hash:  "h2",
			Bytes: 20,
		},
	}
}

// TestTwoBuildsOfOneCorpusAgree is SPEC §18.3's determinism requirement.
//
// The property is load-bearing rather than decorative: §4.6 makes the index
// **per-user**, so two colleagues at one commit must hold indexes that answer the
// same questions, or a disagreement between them could be about their caches rather
// than about the corpus. Nothing tested it.
//
// It is stated over content and not over bytes, and that is the honest form. A
// SQLite file is not byte-stable — page allocation, the freelist, and write order all
// differ between two builds of identical content — so a byte comparison would fail on
// a database that is correct, and a determinism test that fails on correct output is
// a test somebody turns off.
func TestTwoBuildsOfOneCorpusAgree(t *testing.T) {
	t.Parallel()

	first := digestOf(t, docRows())
	second := digestOf(t, docRows())
	if first != second {
		t.Errorf("two builds of one corpus differ:\n%s\n%s", first, second)
	}
	if first == "" {
		t.Error("the digest is empty")
	}
}

// TestWriteOrderDoesNotChangeTheDigest. Rows arrive in whatever order a directory
// walk produced, which §18.3 lists as its own source of non-determinism — so a digest
// that depended on insertion order would fail on two machines with different
// filesystems and pass on one.
func TestWriteOrderDoesNotChangeTheDigest(t *testing.T) {
	t.Parallel()

	forward := docRows()
	reversed := []index.DocumentRow{forward[1], forward[0]}
	if digestOf(t, forward) != digestOf(t, reversed) {
		t.Error("the digest depends on the order rows were written")
	}
}

// TestADifferentCorpusHasADifferentDigest is the negative case, and it is what stops
// a digest that hashes a constant from passing every test above.
func TestADifferentCorpusHasADifferentDigest(t *testing.T) {
	t.Parallel()

	base := digestOf(t, docRows())

	// Each case returns a whole corpus rather than mutating one in place, because
	// removing a row is one of the differences that must show up and a mutation
	// cannot shorten the slice it was handed.
	cases := map[string]func() []index.DocumentRow{
		"a changed title": func() []index.DocumentRow {
			rows := docRows()
			rows[0].Title = "Different"
			return rows
		},
		"a changed hash": func() []index.DocumentRow {
			rows := docRows()
			rows[0].Hash = "other"
			return rows
		},
		"a changed path": func() []index.DocumentRow {
			rows := docRows()
			rows[1].Path = "c/moved.md"
			return rows
		},
		"a changed size": func() []index.DocumentRow {
			rows := docRows()
			rows[1].Bytes = 999
			return rows
		},
		"a removed row": func() []index.DocumentRow { return docRows()[:1] },
	}
	for name, corpus := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := digestOf(t, corpus()); got == base {
				t.Errorf("%s did not change the digest", name)
			}
		})
	}
}

// TestAnEmptyIndexHasAStableDigest. A freshly initialised bundle is the state most
// corpora are in at least once, and two people running `init` must agree.
func TestAnEmptyIndexHasAStableDigest(t *testing.T) {
	t.Parallel()

	empty := digestOf(t, nil)
	if empty != digestOf(t, nil) {
		t.Error("two empty indexes disagree")
	}
	if empty == digestOf(t, docRows()) {
		t.Error("an empty index digests the same as a populated one")
	}
}

// TestEveryContentTableIsDigested is what makes the omission loud.
//
// The failure this design permits is silent: a table added by a later migration and
// not added to the digest would leave its rows out, so two rebuilds differing only in
// that table would report identical digests — a determinism test that passes because
// it is not looking.
//
// FTS5's shadow tables and the indexes are excluded on purpose. They hold the index
// *of* the content rather than the content, their representation is SQLite's business,
// and digesting them would make the answer depend on a library version.
func TestEveryContentTableIsDigested(t *testing.T) {
	t.Parallel()

	db := openTemp(t)
	objects, err := db.Tables(t.Context())
	if err != nil {
		t.Fatalf("tables: %v", err)
	}

	digested := map[string]bool{}
	for _, name := range index.DigestedTables() {
		digested[name] = true
	}
	for _, name := range objects {
		switch {
		case strings.HasSuffix(name, "_fts"), strings.Contains(name, "_fts_"):
			// An FTS5 virtual table or one of its shadow tables. Its content is the
			// table it indexes, which is digested on its own account.
			//
			// Indexes no longer need excluding here: `Tables` asks SQLite for
			// `type = 'table'`, so a new index cannot make this test fail for the
			// wrong reason — which the previous prefix list allowed and did.
			continue
		case digested[name]:
			continue
		default:
			t.Errorf("%s holds content and is not in the digest; two rebuilds "+
				"differing only in that table would report the same digest", name)
		}
	}
}

// digestOf builds a fresh index from rows and reports its digest.
func digestOf(t *testing.T, rows []index.DocumentRow) string {
	t.Helper()

	db := openTemp(t)
	if err := db.Replace(t.Context(), &index.Contents{Documents: rows}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, err := db.Digest(t.Context())
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	return got
}

// TestTheDigestSurvivesARebuild pins the end-to-end shape of §18.3: replacing the
// same rows into an index that already holds them leaves the digest alone, which is
// what makes `index rebuild --check` meaningful.
func TestTheDigestSurvivesARebuild(t *testing.T) {
	t.Parallel()

	db := openTemp(t)
	if err := db.Replace(t.Context(), &index.Contents{Documents: docRows()}); err != nil {
		t.Fatalf("first replace: %v", err)
	}
	first, err := db.Digest(t.Context())
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	if err = db.Replace(t.Context(), &index.Contents{Documents: docRows()}); err != nil {
		t.Fatalf("second replace: %v", err)
	}
	second, err := db.Digest(t.Context())
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	if first != second {
		t.Errorf("rebuilding the same rows changed the digest:\n%s\n%s", first, second)
	}
	// And a document that really moved does change it, or the check above is
	// asserting that Replace does nothing.
	moved := docRows()
	moved[0].Path = "c/renamed.md"
	if err = db.Replace(t.Context(), &index.Contents{Documents: moved}); err != nil {
		t.Fatalf("third replace: %v", err)
	}
	third, err := db.Digest(t.Context())
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	if third == first {
		t.Error("a moved document did not change the digest")
	}
}
