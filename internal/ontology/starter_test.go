package ontology_test

import (
	"testing"

	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/gnosis/internal/ontology"
)

// TestStarterLoads is the property that matters most about the seed: a
// vocabulary its own loader rejects would break every bundle created from it,
// and the failure would appear at `init` time in somebody else's terminal.
func TestStarterLoads(t *testing.T) {
	t.Parallel()

	o, err := ontology.Load(ontology.Starter())
	if err != nil {
		t.Fatalf("the seed vocabulary does not load: %v", err)
	}
	if len(o.Types) != 5 {
		t.Errorf("got %d types, want the 5 of SPEC §5.8", len(o.Types))
	}
	// Phase 1 deliberately ships no subjects: a subject is a vocabulary
	// negotiation, and there is nothing to negotiate until claims disagree.
	if len(o.Subjects) != 0 {
		t.Errorf("the seed declares %d subjects, want none", len(o.Subjects))
	}
}

// TestStarterAliasesResolve checks the seed's aliases are reachable, since an
// alias nothing resolves is decoration.
func TestStarterAliasesResolve(t *testing.T) {
	t.Parallel()

	o, err := ontology.Load(ontology.Starter())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for surface, want := range map[string]gnosis.TypeKey{
		"Runbook":   "Procedure",
		"runbook":   "Procedure",
		"Playbook":  "Procedure",
		"Guideline": "Rule",
		"Note":      "Reference",
		"Reference": "Reference",
	} {
		got, ok := o.ResolveType(gnosis.Surface(surface))
		if !ok {
			t.Errorf("%q resolves to no type", surface)
			continue
		}
		if got != want {
			t.Errorf("%q resolved to %q, want %q", surface, got, want)
		}
	}
}

// TestStarterIsNotSharedMutableState: Starter returns a copy, so one caller
// cannot corrupt the seed for the next.
func TestStarterIsNotSharedMutableState(t *testing.T) {
	t.Parallel()

	first := ontology.Starter()
	if len(first) == 0 {
		t.Fatal("the seed is empty")
	}
	first[0] = '!'

	if _, err := ontology.Load(ontology.Starter()); err != nil {
		t.Fatalf("mutating one copy broke the next: %v", err)
	}
}

// TestStarterTemplatesExistIfDeclared: a type naming a template file that the
// scaffold does not write would make `doctor` report a finding on the day the
// bundle was created. The seed declares none, and this pins that.
func TestStarterTemplatesExistIfDeclared(t *testing.T) {
	t.Parallel()

	o, err := ontology.Load(ontology.Starter())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, ty := range o.Types {
		if ty.Template != "" {
			t.Errorf("type %q declares template %q, which `gnosis init` does not write",
				ty.Key, ty.Template)
		}
	}
}
