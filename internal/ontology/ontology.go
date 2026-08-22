// Package ontology loads and validates the corpus vocabulary: the types every
// document must declare, and the subjects claims may be compared on.
//
// The vocabulary is TOML rather than YAML, and the reason is a property this
// package depends on: toml.Decode reports keys it did not consume, so a mistyped
// flag such as `normatve = true` is caught. Decoding YAML into a map cannot tell
// a typo from a producer-defined extension. For a file a mixed group edits
// during review, a silently ignored flag is the expensive failure. See SPEC §5.2.
//
// Everything here is pure. Load takes bytes; the caller reads the file.
package ontology

import (
	"fmt"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// Type is a declared kind of concept.
type Type struct {
	Key            gnosis.TypeKey `toml:"key"`
	Desc           string         `toml:"desc"`
	Normative      bool           `toml:"normative"`
	ExpectsSubject bool           `toml:"expects_subject"`
	Template       string         `toml:"template"`
	Aliases        []string       `toml:"aliases"`
	Rejected       []Rejection    `toml:"rejected"`
	Deprecated     *Deprecation   `toml:"deprecated"`
}

// Subject is a declared thing claims may bound or describe.
type Subject struct {
	Key                gnosis.SubjectKey `toml:"key"`
	Dimension          Dimension         `toml:"dimension"`
	Desc               string            `toml:"desc"`
	Aliases            []string          `toml:"aliases"`
	Rejected           []Rejection       `toml:"rejected"`
	RequiresCapability bool              `toml:"requires_capability"`
	Deprecated         *Deprecation      `toml:"deprecated"`
}

// Rejection is a surface phrase that was proposed as an alias and declined.
//
// §5.8.2 requires the reason, and the requirement is the point rather than the
// record: an `aliases` list keeps the conclusion and throws away the reasoning, so
// the phrase gets proposed again by somebody who was not in the room when it was
// refused. It is worse here than in most places because §5.8.2.1 makes an alias
// exclusive — admitting one wrongly forecloses a key another group needed.
type Rejection struct {
	// Alias is the phrase that was proposed.
	Alias string `toml:"alias"`

	// Reason is why it was declined, in one sentence. Required: a rejection with
	// no reason records that somebody said no and not what they knew.
	Reason string `toml:"reason"`
}

// Deprecation announces a key's retirement before enforcing it. While Error is
// false a use is reported and nothing breaks, which is what makes a vocabulary
// change survivable once documents already reference the old key.
type Deprecation struct {
	Message string `toml:"message"`
	Error   bool   `toml:"error"`
}

// Ontology is a loaded, validated vocabulary.
type Ontology struct {
	Version  int
	Imports  []string
	Types    []Type
	Subjects []Subject

	typeByAlias    map[string]gnosis.TypeKey
	subjectByAlias map[string]gnosis.SubjectKey
}

// file mirrors the on-disk shape. It exists so the decoded form and the
// validated form are different types: an *Ontology always has its alias indexes
// built, and there is no way to obtain one that does not.
type file struct {
	Version  int       `toml:"version"`
	Imports  []string  `toml:"imports"`
	Types    []Type    `toml:"types"`
	Subjects []Subject `toml:"subjects"`
}

// Load parses and validates a vocabulary.
//
// Requires: src is TOML.
// Ensures: returns EINVALID naming the first problem found — a syntax error, an
// unrecognised key, a duplicate type or subject key, an alias claimed by two
// keys, or an unknown dimension. On success every alias index is populated.
func Load(src []byte) (*Ontology, error) {
	const op = "ontology.Load"

	var f file
	md, err := toml.Decode(string(src), &f)
	if err != nil {
		return nil, &errs.Error{
			Code:    errs.EINVALID,
			Message: op + ": " + err.Error(),
		}
	}
	// A key the decoder did not consume is almost always a typo, and silently
	// ignoring it would leave a flag the author believes is set.
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, 0, len(undecoded))
		for _, k := range undecoded {
			keys = append(keys, k.String())
		}
		sort.Strings(keys)
		return nil, &errs.Error{
			Code:    errs.EINVALID,
			Message: op + ": unrecognised key(s): " + strings.Join(keys, ", "),
		}
	}

	o := &Ontology{
		Version:        f.Version,
		Imports:        f.Imports,
		Types:          f.Types,
		Subjects:       f.Subjects,
		typeByAlias:    map[string]gnosis.TypeKey{},
		subjectByAlias: map[string]gnosis.SubjectKey{},
	}
	if err := o.indexTypes(op); err != nil {
		return nil, err
	}
	if err := o.indexSubjects(op); err != nil {
		return nil, err
	}
	return o, nil
}

// ResolveType maps a surface phrase to a declared type.
//
// Requires: nothing.
// Ensures: matching is fold-insensitive, so "runbook", "Runbook", and "Run book"
// resolve alike. Reports false for an unknown phrase rather than inventing a key.
func (o *Ontology) ResolveType(s gnosis.Surface) (gnosis.TypeKey, bool) {
	k, ok := o.typeByAlias[s.Fold()]
	return k, ok
}

// ResolveSubject maps a surface phrase to a declared subject.
//
// Requires: nothing.
// Ensures: as ResolveType, over the subject vocabulary.
func (o *Ontology) ResolveSubject(s gnosis.Surface) (gnosis.SubjectKey, bool) {
	k, ok := o.subjectByAlias[s.Fold()]
	return k, ok
}

// Identical reports whether two types are behaviourally the same and should
// therefore be one type with two aliases.
//
// Requires: nothing.
// Ensures: compares only what a type actually drives — whether limitations are
// required, whether a missing subject is flagged, and which template applies.
// Descriptions and aliases are excluded deliberately: differing prose is not a
// behavioural difference, and treating it as one would preserve every duplicate
// somebody bothered to describe differently.
func Identical(a, b *Type) bool {
	return a.Normative == b.Normative &&
		a.ExpectsSubject == b.ExpectsSubject &&
		a.Template == b.Template
}

// indexTypes builds the type alias index, rejecting duplicates.
func (o *Ontology) indexTypes(op string) error {
	seen := map[gnosis.TypeKey]bool{}
	for _, t := range o.Types {
		if _, err := gnosis.ParseTypeKey(t.Key.String()); err != nil {
			return &errs.Error{Op: op, Err: err}
		}
		if seen[t.Key] {
			return dup(op, "type", t.Key.String())
		}
		seen[t.Key] = true
		if err := checkRejections(op, "type", t.Key.String(), t.Aliases, t.Rejected); err != nil {
			return err
		}
		for _, s := range append([]string{t.Key.String()}, t.Aliases...) {
			folded := gnosis.Surface(s).Fold()
			if err := claim(o.typeByAlias, folded, t.Key, op, "type"); err != nil {
				return err
			}
		}
	}
	return nil
}

// indexSubjects builds the subject alias index, rejecting duplicates and
// unknown dimensions.
func (o *Ontology) indexSubjects(op string) error {
	seen := map[gnosis.SubjectKey]bool{}
	for _, sub := range o.Subjects {
		if _, err := gnosis.ParseSubjectKey(sub.Key.String()); err != nil {
			return &errs.Error{Op: op, Err: err}
		}
		if seen[sub.Key] {
			return dup(op, "subject", sub.Key.String())
		}
		seen[sub.Key] = true
		if !sub.Dimension.valid() {
			return &errs.Error{
				Code: errs.EINVALID,
				Message: fmt.Sprintf("%s: subject %q has unknown dimension %q",
					op, sub.Key, sub.Dimension),
			}
		}
		if err := checkRejections(op, "subject", sub.Key.String(),
			sub.Aliases, sub.Rejected); err != nil {
			return err
		}
		for _, s := range append([]string{sub.Key.String()}, sub.Aliases...) {
			folded := gnosis.Surface(s).Fold()
			if err := claim(o.subjectByAlias, folded, sub.Key, op, "subject"); err != nil {
				return err
			}
		}
	}
	return nil
}

// claim records an alias, refusing one already taken by a different key.
//
// Two keys sharing a surface phrase is a bounded-context problem the vocabulary
// cannot resolve on the author's behalf (SPEC §5.8.2.1): a key that means two
// things makes every comparison across it either a false contradiction or a
// missed one, and nothing can tell which without asking a person.
//
// The message names the remedy because the obvious repair — deleting one of the
// aliases — is the wrong one. It makes the file load while leaving the ambiguity
// exactly where it was, minus a surface term somebody was using.
func claim[K ~string](index map[string]K, folded string, key K, op, kind string) error {
	if prior, taken := index[folded]; taken && prior != key {
		return &errs.Error{
			Code: errs.EINVALID,
			Message: fmt.Sprintf(
				"%s: %s alias %q is claimed by both %q and %q; "+
					"if they mean the same thing, merge them into one %s with both "+
					"as aliases, and if they mean different things, give each a "+
					"distinct key (%s.%s, %s.%s) and leave %q claimed by neither",
				op, kind, folded, prior, key, kind, prior, folded, key, folded, folded),
		}
	}
	index[folded] = key
	return nil
}

// dup reports a repeated key.
func dup(op, kind, key string) error {
	return &errs.Error{
		Code:    errs.EINVALID,
		Message: fmt.Sprintf("%s: %s %q is declared more than once", op, kind, key),
	}
}
