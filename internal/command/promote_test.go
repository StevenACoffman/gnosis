package command_test

import (
	"strings"
	"testing"

	"github.com/StevenACoffman/gnosis/internal/command"
	"github.com/StevenACoffman/gnosis/internal/gnosis"
	"github.com/StevenACoffman/skillet/errs"
)

// valid is a promotion with every gating field populated, so each test can spoil
// exactly one thing and the reader can see which.
func valid() command.Promote {
	return command.Promote{
		Path:     "c/01932b7c-1f4e-7a3d-9c2b-5e8f0a1b2c3d-cache.md",
		Eff:      command.EffectPreview,
		Approver: "human:priya",
	}
}

func TestValidCommandPasses(t *testing.T) {
	t.Parallel()
	p := valid()
	if err := p.Validate(); err != nil {
		t.Fatalf("a fully populated promotion was rejected: %v", err)
	}
	if p.Op() != "promote" {
		t.Errorf("Op = %q, want promote", p.Op())
	}
	if p.Effect() != command.EffectPreview {
		t.Errorf("Effect = %v, want preview", p.Effect())
	}
}

// TestTheZeroCommandIsRejected is the whole point of the type. A caller that
// forgot to populate the command must not get a write.
func TestTheZeroCommandIsRejected(t *testing.T) {
	t.Parallel()
	var p command.Promote

	err := p.Validate()
	if err == nil {
		t.Fatal("a zero Promote validated")
	}
	if errs.ErrorCode(err) != errs.EINVALID {
		t.Errorf("code = %q, want EINVALID", errs.ErrorCode(err))
	}
	// Every problem at once: a loader that reported one per call would turn a
	// three-field mistake into three round trips.
	for _, want := range []string{"path", "effect", "approver"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestEachGatingFieldIsRequired(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		spoil func(*command.Promote)
		want  string
	}{
		"no path":         {func(p *command.Promote) { p.Path = "" }, "path"},
		"whitespace path": {func(p *command.Promote) { p.Path = "   " }, "path"},
		"unset effect": {
			func(p *command.Promote) { p.Eff = command.EffectUnset },
			"effect is unset",
		},
		"out-of-range effect": {func(p *command.Promote) { p.Eff = command.Effect(99) }, "invalid"},
		"unset approver": {
			func(p *command.Promote) { p.Approver = gnosis.ActorUnset },
			"approver is unset",
		},
		"unprefixed approver": {
			func(p *command.Promote) { p.Approver = "priya" },
			"human, agent, check",
		},
		"unknown actor kind": {
			func(p *command.Promote) { p.Approver = "robot:x" },
			"human, agent, check",
		},
		"required rationale missing": {
			func(p *command.Promote) { p.RequiresRationale = true },
			"requires a rationale",
		},
		"required rationale blank": {
			func(p *command.Promote) { p.RequiresRationale = true; p.Rationale = "  \n " },
			"requires a rationale",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := valid()
			tc.spoil(&p)

			err := p.Validate()
			if err == nil {
				t.Fatalf("%s validated", name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

// TestRationaleIsNotRequiredByDefault: the tier decides, and this package cannot
// know the tier. Requiring one unconditionally would block every promotion at a
// tier that does not ask for it.
func TestRationaleIsNotRequiredByDefault(t *testing.T) {
	t.Parallel()
	p := valid()
	if err := p.Validate(); err != nil {
		t.Errorf("a promotion at a tier requiring no rationale was rejected: %v", err)
	}
}

// TestEffectUnsetIsNotAPreview. Substituting a preview would be almost as bad as
// substituting an apply: a caller that meant to write would believe it had.
func TestEffectUnsetIsNotAPreview(t *testing.T) {
	t.Parallel()
	if command.EffectUnset.Valid() {
		t.Error("EffectUnset validated")
	}
	if command.EffectUnset.Writes() {
		t.Error("EffectUnset writes")
	}
	if command.EffectUnset != 0 {
		t.Error("EffectUnset is not the zero value, so a forgotten field is something else")
	}
}

// TestOnlyApplyWrites: every other value, including one that arrived from a wire
// format, must read as not-writing — the direction in which a mistake is
// recoverable.
func TestOnlyApplyWrites(t *testing.T) {
	t.Parallel()
	if !command.EffectApply.Writes() {
		t.Error("EffectApply does not write")
	}
	for _, e := range []command.Effect{
		command.EffectUnset, command.EffectPreview, command.Effect(-1), command.Effect(7),
	} {
		if e.Writes() {
			t.Errorf("%v writes", e)
		}
	}
}

func TestEffectStringsAreDistinct(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, e := range []command.Effect{
		command.EffectUnset, command.EffectPreview, command.EffectApply, command.Effect(42),
	} {
		s := e.String()
		if s == "" {
			t.Errorf("effect %d renders as empty, so a message would omit it", int(e))
		}
		if seen[s] {
			t.Errorf("two effects both render as %q", s)
		}
		seen[s] = true
	}
}
