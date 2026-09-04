package cmd_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/skillet/proof"
)

// TestProofCreateBindsTheCorpusAndNotTheCache is the property the packet exists for and
// the one it would be quietly wrong about.
//
// A packet that covered `.gnosis/` would publish one person's prompt cache and audit
// trail to whoever the arc was closed for, and would then fail to verify for a colleague
// who had done nothing but run a different query. Both failures are silent at the moment
// they are introduced, which is why this is asserted on the packet rather than on the
// predicate alone.
func TestProofCreateBindsTheCorpusAndNotTheCache(t *testing.T) {
	t.Parallel()

	bundleDir := corpus(t)
	private := filepath.Join(bundleDir, ".gnosis", "audit.jsonl")
	if err := os.MkdirAll(filepath.Dir(private), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(private, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write trail: %v", err)
	}

	out := filepath.Join(t.TempDir(), "packet.json")
	stdout, stderr, err := run(t, "--bundle", bundleDir, "--jsonl",
		"proof", "create", "--arc", "phase-4", "--out", out)
	if err != nil {
		t.Fatalf("proof create: %v\n%s", err, stderr)
	}
	data := decodeData(t, stdout)
	if data["arc"] != "phase-4" {
		t.Errorf("arc = %v, want the one asked for", data["arc"])
	}

	packet := loadPacket(t, out)
	if len(packet.Artifacts) == 0 {
		t.Fatal("the packet declares nothing, which proof.Verify refuses")
	}
	for _, a := range packet.Artifacts {
		if strings.HasPrefix(a.Path, ".gnosis/") {
			t.Errorf("the packet covers per-user state: %s", a.Path)
		}
	}
	// The digests are checked against the bytes, which is the whole claim: a packet
	// listing paths without matching contents proves nothing.
	if vErr := proof.Verify(bundleDir, &packet); vErr != nil {
		t.Errorf("the packet does not verify against the bundle it was made from: %v", vErr)
	}
}

// TestProofCreateSurvivesABundleOutsideGit. A corpus that is not a worktree is a
// supported configuration — `init` does not require one — so refusing to prove it would
// make version control a precondition for integrity rather than for provenance.
func TestProofCreateSurvivesABundleOutsideGit(t *testing.T) {
	t.Parallel()

	out := filepath.Join(t.TempDir(), "packet.json")
	stdout, stderr, err := run(t, "--bundle", corpus(t), "--jsonl",
		"proof", "create", "--arc", "no-git", "--out", out)
	if err != nil {
		t.Fatalf("proof create: %v\n%s", err, stderr)
	}
	if got, ok := decodeData(t, stdout)["git_sha"].(string); !ok || got != "" {
		t.Errorf("git_sha = %v, want the empty string reported rather than omitted", got)
	}
	if packet := loadPacket(t, out); packet.Provenance != nil {
		t.Errorf("provenance = %+v, want none recorded", packet.Provenance)
	}
}

// TestProofCreateRefusesWithoutItsTwoValues. Both are required and neither has a
// defensible default: a packet with no arc names nothing, and one written nowhere cannot
// be verified against later, which is the only thing a packet is for.
func TestProofCreateRefusesWithoutItsTwoValues(t *testing.T) {
	t.Parallel()

	bundleDir := corpus(t)
	for name, args := range map[string][]string{
		"no arc": {"proof", "create", "--out", filepath.Join(t.TempDir(), "p.json")},
		"no out": {"proof", "create", "--arc", "a"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, stderr, err := run(t, append([]string{"--bundle", bundleDir}, args...)...)
			if err == nil {
				t.Fatal("a packet was written with half its inputs")
			}
			if !strings.Contains(stderr, "--arc") || !strings.Contains(stderr, "--out") {
				t.Errorf("the refusal does not name what is missing: %s", stderr)
			}
		})
	}
}

// loadPacket reads a written packet.
func loadPacket(t *testing.T, path string) proof.Packet {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read packet: %v", err)
	}
	var packet proof.Packet
	if uErr := json.Unmarshal(data, &packet); uErr != nil {
		t.Fatalf("the packet is not JSON: %v", uErr)
	}
	return packet
}
