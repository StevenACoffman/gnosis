package standards

import (
	"reflect"
	"sort"
)

// Reading is what a declared value's number actually does.
const (
	// ReadingUnread is the zero value, so a newly declared threshold is reported
	// as doing nothing until somebody says otherwise. The other direction — a new
	// value defaulting to "consumed" — is the silence this whole file exists to
	// break.
	ReadingUnread Reading = iota

	// ReadingConsumed means some code path branches on the number, so editing it
	// in a bundle changes what gnosis does.
	ReadingConsumed

	// ReadingPinned means the value must equal a constant compiled into gnosis and
	// a test enforces that. It is recorded so a record carries the provenance it
	// was produced under, and editing it in a bundle changes nothing except which
	// gnosis will load that bundle.
	//
	// This state exists because the first draft of Unread had only two, and both
	// were wrong for the extractor pair: calling them consumed would tell a reader
	// their edit takes effect, and calling them unread would invite deleting the
	// provenance every extracted record carries.
	ReadingPinned
)

// Reading classifies what reads a declared standards value.
type Reading int

// Declaration is one tunable as this binary declares it: the Go field, the TOML key
// it decodes from, and what reads it.
//
// It exists so the classification can be checked against the source rather than
// against a second copy of itself. `Unread` and `Pinned` answer "which values", which
// a test can only compare to a literal list — and two lists about one fact agree by
// construction and neither is evidence. The Go field name is what a scan of the
// repository can look for, so exporting it turns this file's static map into a claim
// the compiler's own symbols can contradict.
type Declaration struct {
	Field string
	Key   string
	Reads Reading
}

// reading reports what reads a declared value, and naming this switch is the
// point of the file. Nothing at runtime can discover which values are branched
// on, so the knowledge is static and lives here, one function away from the
// structs it describes.
//
// A second place to remember is a smell, and this is one. What makes it tolerable
// is the direction it fails in: a value consumed by new code and not recorded here
// falls to the default and is reported as unread, which is a false alarm somebody
// chases down. The reverse — silently claiming a dead knob is live — is the
// failure that let staleness_days and hedging_max sit unread through two phases,
// and it cannot happen here, because an unrecorded value is a reported one.
func reading(k string) Reading {
	switch k {
	case "allowlist", "per_file_cap", "corpus_budget", "corpus_warn_fraction",
		"embedded_payload_cap", "staleness_days", "hedging_max",
		"rebuild_floor_fraction", "seed":
		// `seed` is consumed by `gnosis debt --sample`, which is the first of the
		// three draws §6.2.1, §10.5, and §14.3.1 specify to be built. It is recorded
		// here as consumed rather than unread because it has a reader today — the
		// point of this file is that the answer is about this binary, not about what
		// the spec eventually wants.
		return ReadingConsumed
	case "html_extractor", "html_extractor_version":
		return ReadingPinned
	default:
		// in_degree_cut is the only value here today. §14.4.1 wants it for the
		// conjunction "unprovable AND load-bearing", and `unprovable` is Phase 3, so
		// the cut has nothing to narrow yet. Giving it a reader that classified bare
		// centrality would be a different feature wearing the same number, and would
		// make the value look consumed while the thing it was declared for stayed
		// unbuilt. Reported instead.
		return ReadingUnread
	}
}

// String renders a reading for a diagnostic.
func (r Reading) String() string {
	switch r {
	case ReadingConsumed:
		return "consumed"
	case ReadingPinned:
		return "pinned"
	case ReadingUnread:
		return "unread"
	default:
		return "invalid"
	}
}

// Declarations is every declared tunable with its Go field and its classification.
//
// Requires: nothing.
// Ensures: one entry per exported field of every standards struct, in declaration
// order, which is a property of this binary and identical for every corpus. Pure.
func Declarations() []Declaration {
	var out []Declaration
	for _, cfg := range []any{Archive{}, Promote{}, Sample{}} {
		t := reflect.TypeOf(cfg)
		for i := range t.NumField() {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			k := key(&f)
			out = append(out, Declaration{Field: f.Name, Key: k, Reads: reading(k)})
		}
	}
	return out
}

// Unread names every declared value that no code branches on.
//
// Requires: nothing. It reads struct tags and a static map, not a bundle, so the
// answer is a property of this binary and identical for every corpus.
// Ensures: sorted, and empty only when every declared value is consumed or pinned.
// A value in neither state is named whether it is absent from the map or recorded
// as unread, so forgetting to classify a new threshold is reported rather than
// waved through.
func Unread() []string {
	var out []string
	for _, k := range declared() {
		if r := reading(k); r != ReadingConsumed && r != ReadingPinned {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// Pinned names every declared value that must match a constant in this binary.
//
// Requires: nothing.
// Ensures: sorted. A caller reporting Unread should report these too and say they
// differ, because the reader's next question about a value nothing branches on is
// whether to delete it, and for these the answer is no.
func Pinned() []string {
	var out []string
	for _, k := range declared() {
		if reading(k) == ReadingPinned {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// declared lists the top-level keys of every standards file, from the structs the
// loaders decode into rather than from a list. A threshold added to a struct is
// therefore classified or reported without anybody remembering this file exists,
// which is the half of the second-place problem that can be automated.
func declared() []string {
	var out []string
	for _, cfg := range []any{Archive{}, Promote{}, Sample{}} {
		t := reflect.TypeOf(cfg)
		for i := range t.NumField() {
			f := t.Field(i)
			if f.IsExported() {
				out = append(out, key(&f))
			}
		}
	}
	return out
}

// Tuned names every unread value this bundle has moved off the seed.
//
// Requires: a and p are loaded standards, either of which may be nil for a bundle
// that declares only one file.
// Ensures: sorted, and empty for a bundle holding the seed values. Pinned values
// are not reported here — a caller that can compare them to the constants they
// pin should do that instead, and this package cannot, since the constants live
// with the code that stamps them.
//
// This is the reportable half of Unread, and the distinction took a failing test
// to find. `doctor` describes the apparatus around one corpus, and "gnosis
// declares a knob it does not read" is a fact about the binary — identical for
// every corpus, actionable by nobody holding one, and not even fixable by
// deleting the value, since the loader then rejects the file for the missing
// rationale. What is genuinely this corpus's business is that somebody edited a
// number here and got nothing for it. The unconditional fact belongs in gnosis's
// own tests, where its only possible audience reads.
func Tuned(a *Archive, p *Promote) []string {
	unread := make(map[string]bool)
	for _, k := range Unread() {
		unread[k] = true
	}
	seedA, errA := LoadArchive(DefaultArchive())
	seedP, errP := LoadPromote(DefaultPromote())
	if errA != nil || errP != nil {
		// The seed not loading is a build-time defect a test already catches, and
		// there is nothing to compare against. Report nothing rather than report
		// every value as tuned.
		return nil
	}
	var out []string
	out = append(out, differing(a, seedA, unread)...)
	out = append(out, differing(p, seedP, unread)...)
	sort.Strings(out)
	return out
}

// differing names the keys of got whose value differs from want, restricted to
// the given set. Both must be the same type; a nil got reports nothing, since a
// bundle that declares no file has tuned nothing.
func differing(got, want any, only map[string]bool) []string {
	g, w := reflect.ValueOf(got), reflect.ValueOf(want)
	if g.Kind() == reflect.Pointer {
		if g.IsNil() || w.IsNil() {
			return nil
		}
		g, w = g.Elem(), w.Elem()
	}
	var out []string
	t := g.Type()
	for i := range g.NumField() {
		f := t.Field(i)
		k := key(&f)
		if !f.IsExported() || !only[k] {
			continue
		}
		// Compare the number, not the rationale: rewording the justification for a
		// value nobody reads is not a finding, and reporting it as one would train
		// a reader to ignore the category.
		if !reflect.DeepEqual(g.Field(i).FieldByName("Value").Interface(),
			w.Field(i).FieldByName("Value").Interface()) {
			out = append(out, k)
		}
	}
	return out
}
