package constraint_test

import (
	"testing"

	"github.com/StevenACoffman/gnosis/internal/constraint"
	"github.com/StevenACoffman/gnosis/internal/standards"
)

// shipped is the pattern set the seed carries, so these cases test the data as well as
// the code. A phrase deleted from operators.toml fails here, which is what makes the file
// a checked artifact rather than a suggestion.
func shipped(t *testing.T) []constraint.Pattern {
	t.Helper()
	in, err := standards.LoadOperators(standards.DefaultOperators())
	if err != nil {
		t.Fatalf("the shipped operator patterns do not load: %v", err)
	}
	out := make([]constraint.Pattern, 0, len(in.Pattern))
	for _, p := range in.Pattern {
		out = append(out, constraint.Pattern{
			ID: p.ID, Phrase: p.Phrase, Op: constraint.OpKind(p.Op),
		})
	}
	return out
}

// TestInversionsAreReadAsWrittenIsTheWholePoint is the adversarial case and the reason
// this file exists at all.
//
// "no fewer than three" and "no more than three" differ by one word and invert the
// operator. A matcher that stopped at its first hit, or that sorted by declaration order,
// would read a floor as a ceiling — and the resulting conflict finding would be confident
// and backwards.
func TestInversionsAreReadAsWrittenIsTheWholePoint(t *testing.T) {
	t.Parallel()
	patterns := shipped(t)

	cases := map[string]struct {
		op    constraint.OpKind
		value float64
	}{
		"Retries must be no fewer than three.":     {constraint.OpAtLeast, 3},
		"Retries must be no more than three.":      {constraint.OpAtMost, 3},
		"Retries should not exceed three.":         {constraint.OpAtMost, 3},
		"The budget is no less than 5 attempts.":   {constraint.OpAtLeast, 5},
		"Latency must be under 400 milliseconds.":  {constraint.OpAtMost, 400},
		"Availability is over 99.9%.":              {constraint.OpAtLeast, 99.9},
		"The timeout is exactly two and a half s.": {constraint.OpExactly, 2.5},
		"The retry budget is 3.":                   {constraint.OpExactly, 3},
	}
	for text, want := range cases {
		t.Run(text, func(t *testing.T) {
			t.Parallel()
			got, ok := constraint.Parse(text, patterns)
			if !ok {
				t.Fatalf("no constraint parsed from %q", text)
			}
			if got.Op != want.op || got.Value != want.value {
				t.Errorf("parsed %s, want {op: %s, value: %v}",
					got, want.op, want.value)
			}
		})
	}
}

// TestProseThatStatesNoConstraintParsesToNothing is the negative half of the corpus, and
// a pattern set without one will happily read a number out of prose that bounds nothing.
//
// The "is" pattern is the loosest in the file and most of these are about it: it has to
// catch "the retry budget is 3" without catching every sentence containing the word.
func TestProseThatStatesNoConstraintParsesToNothing(t *testing.T) {
	t.Parallel()
	patterns := shipped(t)

	for _, text := range []string{
		// Sentence-shaped, with terminal punctuation, because that is what a claim
		// anchor is. The first version of this corpus omitted it and passed while the
		// commonest real form failed.
		"The retry budget is generous.",
		"Retries are capped, and the cap is documented elsewhere.",
		// The article trap: numwords reads "a" as the number one, so without a guard
		// this parses to a confident `<= 1` on prose that bounds nothing. §7.3 pinned
		// that library for floats and fractions and this is the cost — in English the
		// article and the numeral are the same word.
		"No more than a handful of retries.",
		"At least an hour of retries.",
		"This document is about retries.",
		"Backoff is exponential.",
		"There is no constraint on retries.",
	} {
		t.Run(text, func(t *testing.T) {
			t.Parallel()
			if got, ok := constraint.Parse(text, patterns); ok {
				t.Errorf("prose stating no constraint parsed to %s", got)
			}
		})
	}
}

// TestTheFirstNumberAfterThePhraseWins keeps a two-quantity sentence from being bounded
// on the wrong one. "no more than 3 retries in 60 seconds" carries two numbers, and a
// parser taking the last would silently constrain the window instead of the count.
func TestTheFirstNumberAfterThePhraseWins(t *testing.T) {
	t.Parallel()
	got, ok := constraint.Parse("No more than 3 retries in 60 seconds.", shipped(t))
	if !ok {
		t.Fatal("nothing parsed")
	}
	if got.Value != 3 {
		t.Errorf("bounded %v, want 3 — the number the phrase introduced", got.Value)
	}
}

// TestAConstraintShowsItsParse is §10.2.2's requirement reaching the type: an adjudicator
// sees the reading beside the claim, because a false conflict that shows its reasoning is
// dismissible in seconds and one that shows a verdict erodes the queue.
func TestAConstraintShowsItsParse(t *testing.T) {
	t.Parallel()
	got, ok := constraint.Parse("Retries must be no more than three.", shipped(t))
	if !ok {
		t.Fatal("nothing parsed")
	}
	if got.String() != "{op: <=, value: 3}" {
		t.Errorf("String() = %q, which is not a parse a reader can check", got.String())
	}
	if got.PatternID == "" {
		t.Error("the reading does not say which pattern produced it")
	}
}

// TestNoPatternsParsesNothing is the degradation path: a corpus whose operator file will
// not load reads no constraints rather than reading them wrongly, which leaves the
// interval and enumeration predicates skipped instead of confident.
func TestNoPatternsParsesNothing(t *testing.T) {
	t.Parallel()
	if got, ok := constraint.Parse("No more than three retries.", nil); ok {
		t.Errorf("an empty pattern set produced %s", got)
	}
}

// TestADecimalPointIsNotSentencePunctuation is §9.4's own naive-splitting list one layer
// down: that section records that `split(".")` cuts `2.5 seconds` in half, and spacing a
// decimal point does the same damage. The first version of the punctuation pass turned
// "over 99.9%" into a bound of 99 — a confident number, wrong by an order of magnitude in
// the direction that matters.
func TestADecimalPointIsNotSentencePunctuation(t *testing.T) {
	t.Parallel()
	patterns := shipped(t)

	for text, want := range map[string]float64{
		"Availability is over 99.9%.":        99.9,
		"The timeout is at most 2.5 seconds": 2.5,
		"Retries are at most 1,000 per hour": 1000,
	} {
		t.Run(text, func(t *testing.T) {
			t.Parallel()
			got, ok := constraint.Parse(text, patterns)
			if !ok {
				t.Fatalf("nothing parsed from %q", text)
			}
			if got.Value != want {
				t.Errorf("bounded %v, want %v — a number split by the punctuation pass",
					got.Value, want)
			}
		})
	}
}

// TestAUnitWrittenAgainstItsNumberParses is §18.4.1's failure recurring one field over,
// and the case every case in this corpus was missing.
//
// A latency budget is written "400ms" and a size cap "5MB" — nobody writes "400 ms" — so
// `duration` and `bytes`, two of the four declared dimensions (§10.2.1), had no working
// ordinary form. Every case here was spaced, so thirteen patterns and a green suite said
// nothing about it. This is the same shape as `99.9%` splitting into 99, recorded in
// §18.4.1 two sections from where it happened again.
func TestAUnitWrittenAgainstItsNumberParses(t *testing.T) {
	t.Parallel()

	cases := []struct {
		sentence string
		op       string
		value    float64
		raw      string
	}{
		{"The timeout must be under 400ms.", "<=", 400, "400ms"},
		{"The payload must not exceed 5MB.", "<=", 5, "5mb"},
		{"The budget must be at least 2.5s.", ">=", 2.5, "2.5s"},
		// The spaced form must keep working; the fix is an addition, not a swap.
		{"The timeout must be under 400 ms.", "<=", 400, "400"},
		// And the punctuation case §18.4.1 already fixed must not regress.
		{"Availability must be at least 99.9%.", ">=", 99.9, "99.9%"},
	}
	for _, c := range cases {
		got, ok := constraint.Parse(c.sentence, shipped(t))
		if !ok {
			t.Errorf("%q parsed to nothing", c.sentence)
			continue
		}
		if string(got.Op) != c.op || got.Value != c.value {
			t.Errorf("%q -> {op: %s, value: %v}, want {op: %s, value: %v}",
				c.sentence, got.Op, got.Value, c.op, c.value)
		}
		// The unit survives in Raw, which is the only record that the claim said
		// "ms" at all — the dimension it is compared under comes from the subject's
		// declaration, so a claim writing "400ms" about a `count` subject is a
		// category error nothing else could see.
		if got.Raw != c.raw {
			t.Errorf("%q raw = %q, want %q", c.sentence, got.Raw, c.raw)
		}
	}
}

// TestAWordIsStillNotAQuantity is the adversarial half of the fix above. Stripping
// trailing letters must not turn prose into a number: "a handful" states no constraint,
// and "v2" is a name rather than a bound.
func TestAWordIsStillNotAQuantity(t *testing.T) {
	t.Parallel()

	for _, s := range []string{
		"Retries must be no more than a handful.",
		"The version must be no more than v2.",
		"The limit must be under discussion.",
	} {
		if got, ok := constraint.Parse(s, shipped(t)); ok {
			t.Errorf("%q parsed to %+v; it states no quantity", s, got)
		}
	}
}
