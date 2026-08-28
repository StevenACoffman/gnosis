package ontology_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/ontology"
	"github.com/StevenACoffman/skillet/errs"
)

const minimal = `
version = 1

[[types]]
key = "Reference"
desc = "a recorded fact"
normative = false
expects_subject = false
aliases = ["Note", "Background"]

[[subjects]]
key = "retry.max_attempts"
dimension = "count"
desc = "attempts before abandoning"
aliases = ["retry budget", "retry cap"]
`

func TestLoadMinimal(t *testing.T) {
	t.Parallel()
	o, err := ontology.Load([]byte(minimal))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(o.Types) != 1 || len(o.Subjects) != 1 {
		t.Fatalf("got %d types and %d subjects, want 1 and 1", len(o.Types), len(o.Subjects))
	}
}

// TestResolveIsFoldInsensitive is the property that lets each function write in
// its own words: engineering's "retry budget" and support's "Retry  Cap" must
// reach the same key without anyone renaming anything.
func TestResolveIsFoldInsensitive(t *testing.T) {
	t.Parallel()
	o, err := ontology.Load([]byte(minimal))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	subjects := []string{
		"retry.max_attempts", "retry budget", "Retry Budget",
		"RETRY CAP", "retry   cap",
	}
	for _, s := range subjects {
		got, ok := o.ResolveSubject(gnosis.Surface(s))
		if !ok {
			t.Errorf("ResolveSubject(%q) not found", s)
			continue
		}
		if got != "retry.max_attempts" {
			t.Errorf("ResolveSubject(%q) = %q, want retry.max_attempts", s, got)
		}
	}

	for _, s := range []string{"Reference", "note", "BACKGROUND"} {
		if got, ok := o.ResolveType(gnosis.Surface(s)); !ok || got != "Reference" {
			t.Errorf("ResolveType(%q) = %q, %v; want Reference, true", s, got, ok)
		}
	}

	if _, ok := o.ResolveSubject("nothing declared"); ok {
		t.Error("an undeclared phrase resolved; want false rather than an invented key")
	}
}

// TestLoadRejectsTypoedKey is the reason this file is TOML rather than YAML.
// Decoding YAML into a map cannot distinguish `normatve` from a producer's own
// extension, so the flag would be silently unset and the author would believe
// otherwise. toml.Decode reports it.
func TestLoadRejectsTypoedKey(t *testing.T) {
	t.Parallel()
	src := "version = 1\n[[types]]\nkey = \"A\"\nnormatve = true\n"
	_, err := ontology.Load([]byte(src))
	if err == nil {
		t.Fatal("Load accepted a mistyped key")
	}
	if !strings.Contains(err.Error(), "normatve") {
		t.Errorf("diagnostic %q does not name the offending key", err)
	}
}

func TestLoadRejects(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"duplicate type key": "version = 1\n" +
			"[[types]]\nkey = \"A\"\n[[types]]\nkey = \"A\"\n",
		"duplicate subject key": "version = 1\n" +
			"[[subjects]]\nkey = \"a.b\"\ndimension = \"count\"\n" +
			"[[subjects]]\nkey = \"a.b\"\ndimension = \"count\"\n",
		"alias claimed twice": "version = 1\n" +
			"[[types]]\nkey = \"A\"\naliases = [\"shared\"]\n" +
			"[[types]]\nkey = \"B\"\naliases = [\"shared\"]\n",
		"unknown dimension": "version = 1\n" +
			"[[subjects]]\nkey = \"a.b\"\ndimension = \"furlongs\"\n",
		"missing dimension": "version = 1\n[[subjects]]\nkey = \"a.b\"\n",
		"empty type key":    "version = 1\n[[types]]\nkey = \"\"\n",
		"spaced subject key": "version = 1\n" +
			"[[subjects]]\nkey = \"a b\"\ndimension = \"count\"\n",
		"syntax error":  "version = 1\n[[types]\nkey = \"A\"\n",
		"duplicate top": "version = 1\nversion = 2\n",
	}
	for name, src := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := ontology.Load([]byte(src))
			if err == nil {
				t.Fatalf("Load accepted an invalid vocabulary:\n%s", src)
			}
			if code := errs.ErrorCode(err); code != errs.EINVALID {
				t.Errorf("code = %q, want %q", code, errs.EINVALID)
			}
		})
	}
}

// TestAKeyMayRepeatItsOwnAlias guards the boundary of the alias-collision rule:
// two keys sharing a phrase is an error, but a key listing its own name is not.
func TestAKeyMayRepeatItsOwnAlias(t *testing.T) {
	t.Parallel()
	src := "version = 1\n[[types]]\nkey = \"Rule\"\naliases = [\"Rule\", \"rule\"]\n"
	if _, err := ontology.Load([]byte(src)); err != nil {
		t.Errorf("Load rejected a key repeating its own name: %v", err)
	}
}

// TestAliasCollisionNamesTheRemedy pins what SPEC §5.8.2.1 requires of this
// diagnostic, not merely that it fires.
//
// The rule forces a conversation, so the message has to say what the conversation
// is about. Without it the obvious repair is to delete one alias — which makes the
// file load while leaving the ambiguity exactly where it was, minus a surface term
// somebody was using. Both branches must be offered, because the reader is the only
// one who knows whether the two keys mean the same thing.
func TestAliasCollisionNamesTheRemedy(t *testing.T) {
	t.Parallel()
	src := "version = 1\n" +
		"[[types]]\nkey = \"A\"\naliases = [\"shared\"]\n" +
		"[[types]]\nkey = \"B\"\naliases = [\"shared\"]\n"

	_, err := ontology.Load([]byte(src))
	if err == nil {
		t.Fatal("two keys claiming one alias were accepted")
	}
	msg := err.Error()

	for _, want := range []string{
		"shared", // the offending phrase
		"\"A\"",  // both claimants, so the reader need not go looking
		"\"B\"",
		"merge",    // the same-thing branch
		"distinct", // the different-things branch
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("diagnostic does not mention %q:\n%s", want, msg)
		}
	}
}

// TestIdenticalComparesBehaviourNotProse encodes the merge rule from SPEC
// §5.8.1: two types are the same type when they drive the same behaviour, so
// differing descriptions must not keep a duplicate alive.
func TestIdenticalComparesBehaviourNotProse(t *testing.T) {
	t.Parallel()
	base := ontology.Type{
		Key: "Procedure", Desc: "steps", Normative: true,
		ExpectsSubject: false, Template: "t.md",
	}
	tests := map[string]struct {
		other ontology.Type
		want  bool
	}{
		"same behaviour, different prose": {
			ontology.Type{
				Key:       "Runbook",
				Desc:      "an operational procedure",
				Normative: true,
				Template:  "t.md",
			},
			true,
		},
		"same behaviour, different aliases": {
			ontology.Type{
				Key:       "Playbook",
				Normative: true,
				Template:  "t.md",
				Aliases:   []string{"x"},
			},
			true,
		},
		"normative differs": {
			ontology.Type{Key: "X", Normative: false, Template: "t.md"},
			false,
		},
		"expects_subject differs": {
			ontology.Type{Key: "X", Normative: true, ExpectsSubject: true, Template: "t.md"},
			false,
		},
		"episodic differs": {
			ontology.Type{Key: "X", Normative: true, Episodic: true, Template: "t.md"},
			false,
		},
		"template differs": {
			ontology.Type{Key: "X", Normative: true, Template: "other.md"},
			false,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := ontology.Identical(&base, &tc.other); got != tc.want {
				t.Errorf("Identical = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestEveryBehaviouralFlagReachesIdentical is the case the hand-written table above
// cannot cover: the one for a flag nobody has added yet.
//
// `Identical` decides whether two declarations of one type key are the same type
// (§5.8.2). A flag added to Type and forgotten here makes two types whose behaviour
// differs compare identical, and the merge that follows is silent — no error, no
// finding, just a type that quietly stops driving the check it was declared for.
//
// So this enumerates the bool fields off the struct itself rather than listing them.
// It is the same cross-check `standards.Unread` gets, for the same reason: the list
// and the truth lived in two places once before, and the truth moved.
func TestEveryBehaviouralFlagReachesIdentical(t *testing.T) {
	t.Parallel()

	// Desc-like fields are excluded by design, not by oversight: differing prose is
	// not a behavioural difference. Any bool, though, is a flag driving something.
	base := ontology.Type{Key: "X", Template: "t.md"}
	typ := reflect.TypeOf(base)

	for i := range typ.NumField() {
		field := typ.Field(i)
		if field.Type.Kind() != reflect.Bool {
			continue
		}
		t.Run(field.Name, func(t *testing.T) {
			t.Parallel()
			other := base
			reflect.ValueOf(&other).Elem().Field(i).SetBool(true)

			if ontology.Identical(&base, &other) {
				t.Errorf("Identical ignores %s: two types differing only in that flag "+
					"compare as one, so §5.8.2 would merge declarations whose "+
					"behaviour differs", field.Name)
			}
		})
	}
}
