package scan_test

import (
	"testing"

	"github.com/StevenACoffman/gnosis/internal/scan"
)

// Every fixture here writes its hidden characters as escape sequences rather
// than as literals, and not only because Go refuses a literal byte-order mark in
// source. A test for invisible characters whose fixtures are invisible is a test
// nobody can review, which is the same failure the check itself exists to catch.
// Written this way, a reader can see which codepoint each case covers.

// TestEachClassIsDetected covers every codepoint SPEC 9.3 names. The constants
// are ranges from the Unicode standard rather than tuned thresholds, which is
// what makes them safe to block on, so this enumerates them rather than sampling.
func TestEachClassIsDetected(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		text  string
		class scan.Class
		rune  string
	}{
		"zero-width space":        {"ordinary\U0000200Btext", scan.ClassZeroWidth, "U+200B"},
		"zero-width non-joiner":   {"ordinary\U0000200Ctext", scan.ClassZeroWidth, "U+200C"},
		"zero-width joiner":       {"ordinary\U0000200Dtext", scan.ClassZeroWidth, "U+200D"},
		"word joiner":             {"ordinary\U00002060text", scan.ClassZeroWidth, "U+2060"},
		"byte order mark":         {"ordinary\U0000FEFFtext", scan.ClassZeroWidth, "U+FEFF"},
		"left-to-right embedding": {"a\U0000202Ab", scan.ClassBidi, "U+202A"},
		"right-to-left embedding": {"a\U0000202Bb", scan.ClassBidi, "U+202B"},
		"pop directional":         {"a\U0000202Cb", scan.ClassBidi, "U+202C"},
		"left-to-right override":  {"a\U0000202Db", scan.ClassBidi, "U+202D"},
		"right-to-left override":  {"a\U0000202Eb", scan.ClassBidi, "U+202E"},
		"left-to-right isolate":   {"a\U00002066b", scan.ClassBidi, "U+2066"},
		"right-to-left isolate":   {"a\U00002067b", scan.ClassBidi, "U+2067"},
		"first strong isolate":    {"a\U00002068b", scan.ClassBidi, "U+2068"},
		"pop directional isolate": {"a\U00002069b", scan.ClassBidi, "U+2069"},
		"tag block start":         {"a\U000E0001b", scan.ClassTag, "U+E0001"},
		"tag block end":           {"a\U000E007Fb", scan.ClassTag, "U+E007F"},
		// Listed since 2026-11-02, when gnosis took skillet's shared classifier. These
		// were asserted legitimate before; see TODO.md for the change of position.
		"soft hyphen":               {"ig\U000000ADnore", scan.ClassZeroWidth, "U+00AD"},
		"mongolian vowel separator": {"a\U0000180Eb", scan.ClassZeroWidth, "U+180E"},
		"function application":      {"a\U00002061b", scan.ClassZeroWidth, "U+2061"},
		"invisible times":           {"a\U00002062b", scan.ClassZeroWidth, "U+2062"},
		"invisible separator":       {"a\U00002063b", scan.ClassZeroWidth, "U+2063"},
		"invisible plus":            {"a\U00002064b", scan.ClassZeroWidth, "U+2064"},
		"arabic letter mark":        {"a\U0000061Cb", scan.ClassBidi, "U+061C"},
		"left-to-right mark":        {"a\U0000200Eb", scan.ClassBidi, "U+200E"},
		"right-to-left mark":        {"a\U0000200Fb", scan.ClassBidi, "U+200F"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := scan.Hidden(tc.text)
			if len(got) != 1 {
				t.Fatalf("got %d findings, want 1: %+v", len(got), got)
			}
			if got[0].Class != tc.class {
				t.Errorf("class = %q, want %q", got[0].Class, tc.class)
			}
			if got[0].Rune != tc.rune {
				t.Errorf("rune = %q, want %q", got[0].Rune, tc.rune)
			}
		})
	}
}

// TestOrdinaryTextIsClean, or the check is a nuisance rather than a guard. These
// look suspicious and are not: a non-breaking space is nearly invisible and entirely
// legitimate, three cases sit one codepoint outside a flagged range, and the last four
// are the legitimate uses of listed codepoints that the shared classifier excuses by
// context — refusing them refused emoji, Persian text and the England flag.
func TestOrdinaryTextIsClean(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"empty":              "",
		"plain ascii":        "The cache is cleared on restart.",
		"accented latin":     "caf\U000000E9 naive resume",
		"cjk":                "\U000030AD\U000030E3",
		"emoji":              "shipped \U0001F680 today",
		"arabic":             "\U00000627\U00000644",
		"non-breaking space": "one\U000000A0two",
		"just below 200B":    "a\U0000200Ab",
		"just above 2064":    "a\U00002065b",
		"just below E0001":   "a\U000E0000b",
		"emoji ZWJ sequence": "pair \U0001F469\U0000200D\U0001F4BB today",
		"persian ZWNJ":       "\U00000645\U000006CC\U0000200C\U0000062E\U00000648\U00000627\U00000647\U00000645",
		"england flag": "go \U0001F3F4\U000E0067\U000E0062\U000E0065\U000E006E" +
			"\U000E0067\U000E007F team",
		"leading byte order mark": "\U0000FEFFtitle",
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := scan.Hidden(text); len(got) != 0 {
				t.Errorf("clean text reported %+v", got)
			}
		})
	}
}

// TestTrojanSource is the attack the bidi class exists for: what a reviewer
// reads is not what a parser or a model consumes.
func TestTrojanSource(t *testing.T) {
	t.Parallel()
	src := "if accessLevel != \"user" +
		"\U0000202E" + "\U00002066" + "// Check if admin" + "\U00002069"

	got := scan.Hidden(src)
	if len(got) != 1 || got[0].Class != scan.ClassBidi {
		t.Fatalf("Trojan Source not detected: %+v", got)
	}
	if got[0].Count != 3 {
		t.Errorf("count = %d, want every override counted", got[0].Count)
	}
}

// TestOffsetIsBytes, matching claims.pos and links.snippet_start (5.5.2). A rune
// offset would send a reader to the wrong place in any document containing an
// accent, which is most of them.
func TestOffsetIsBytes(t *testing.T) {
	t.Parallel()
	// The e-acute is two bytes, so "caf" plus it is five bytes and four runes.
	got := scan.Hidden("caf\U000000E9\U0000200B")

	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1", len(got))
	}
	if got[0].Offset != 5 {
		t.Errorf("offset = %d, want 5 bytes rather than 4 runes", got[0].Offset)
	}
}

// TestEmptyNotNil, so a caller need not distinguish "no findings" from "did not
// run"; that distinction is what Stages is for.
func TestEmptyNotNil(t *testing.T) {
	t.Parallel()
	if got := scan.Hidden("clean"); got == nil {
		t.Error("clean text returned nil rather than an empty slice")
	}
}

// TestCoverageOfReportsWhatRan, not what SPEC 9.3 specifies. A caller reporting a
// clean scan must be able to say which stages produced it, because "no hidden
// characters" and "9.3 passed" are different claims and only one is available.
func TestCoverageOfReportsWhatRan(t *testing.T) {
	t.Parallel()
	got := scan.CoverageOf(scan.StageHidden)
	if len(got.Ran) != 1 || got.Ran[0] != scan.StageHidden {
		t.Errorf("Ran = %v, want only the implemented stage", got.Ran)
	}
	if len(got.Missing) != 3 {
		t.Errorf("Missing = %v, want the three unbuilt stages", got.Missing)
	}
}

// TestAnIncompleteScanSaysSo. Complete is what the promote gate branches on, so a
// partial scan reading as complete would let a candidate through on one stage of
// four.
func TestAnIncompleteScanSaysSo(t *testing.T) {
	t.Parallel()
	if scan.CoverageOf(scan.StageHidden).Complete() {
		t.Error("a scan running one stage of four reported itself complete")
	}
	full := scan.Coverage{Ran: []string{scan.StageHidden}}
	if !full.Complete() {
		t.Error("a scan with nothing missing did not report complete")
	}
}

// TestTheZeroCoverageIsNotComplete. Same discipline as every other zero value
// here: a value nobody populated must not assert that everything was checked.
func TestTheZeroCoverageIsNotComplete(t *testing.T) {
	t.Parallel()
	if (scan.Coverage{}).Complete() {
		t.Error("the zero Coverage reports a complete scan")
	}
}

// TestTheZeroClassNamesNothing, so a Finding nobody populated cannot be mistaken
// for a report about zero-width characters.
func TestTheZeroClassNamesNothing(t *testing.T) {
	t.Parallel()
	var c scan.Class
	if c != scan.ClassUnset {
		t.Errorf("the zero Class is %q, not unset", c)
	}
}
