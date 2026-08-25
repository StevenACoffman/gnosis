package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// agentsPath is the schema document in a bundle.
func agentsPath(dir string) string { return filepath.Join(dir, "AGENTS.md") }

// readFile reads a file, treating absence as empty.
func readFile(t *testing.T, path string) string {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}

// TestAHandWrittenSchemaFileIsNeverOverwritten is the fail-closed rule, through the
// real command.
//
// §5.7 records the cost of getting this wrong: an ETH Zurich study found
// auto-generated context files *reduced* agent success in five of eight settings. A
// file predating the tool was not written under its contract, so converting it
// silently is a change with measured evidence against it.
func TestAHandWrittenSchemaFileIsNeverOverwritten(t *testing.T) {
	t.Parallel()
	dir := corpus(t)

	const mine = "# Agents\n\nRead the retry budget page first. Ask Priya about the\n" +
		"queue.   \n"
	if err := os.WriteFile(agentsPath(dir), []byte(mine), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	stdout, stderr, err := run(t, "--bundle", dir, "schema")
	if err == nil {
		t.Error("a file that could not be updated reported success")
	}
	// Byte for byte, including the trailing spaces a formatter would remove.
	if got := readFile(t, agentsPath(dir)); got != mine {
		t.Errorf("the hand-written file changed:\n%q", got)
	}
	sibling := readFile(t, filepath.Join(dir, "AGENTS.generated.md"))
	if !strings.Contains(sibling, "gnosis:begin vocabulary") {
		t.Errorf("the generated text was not written beside it:\n%s", sibling)
	}
	if !strings.Contains(stderr, "carries no gnosis markers") {
		t.Errorf("the report does not say why:\n%s\n%s", stdout, stderr)
	}
}

// TestAnUnterminatedMarkerWritesNothing is the fourth rule the backlog entry does not
// state: one typo must not hand a whole document to a generator.
func TestAnUnterminatedMarkerWritesNothing(t *testing.T) {
	t.Parallel()
	dir := corpus(t)

	const damaged = "# Agents\n\n<!-- gnosis:begin vocabulary -->\n- Reference\n\n" +
		"My own notes, which follow the region and are not part of it.\n"
	if err := os.WriteFile(agentsPath(dir), []byte(damaged), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, stderr, err := run(t, "--bundle", dir, "schema")
	if err == nil {
		t.Fatal("a file with an unterminated marker was written")
	}
	if got := readFile(t, agentsPath(dir)); got != damaged {
		t.Errorf("the damaged file was modified:\n%q", got)
	}
	// The diagnostic names the marker, because a caller told "malformed" with no name
	// has to search the file.
	if !strings.Contains(stderr, "vocabulary") {
		t.Errorf("the refusal does not name the marker: %s", stderr)
	}
	// And nothing was written beside it either: this is not the unmarked case, and a
	// sibling file would suggest the fix is to paste rather than to close the marker.
	if got := readFile(t, filepath.Join(dir, "AGENTS.generated.md")); got != "" {
		t.Errorf("a malformed file produced a sibling:\n%s", got)
	}
}

// TestTheGeneratedRegionsAreWrittenAndUpdated is the ordinary path, twice: once into
// an absent file and once over a stale region.
func TestTheGeneratedRegionsAreWrittenAndUpdated(t *testing.T) {
	t.Parallel()
	dir := corpus(t)

	if _, stderr, err := run(t, "--bundle", dir, "schema"); err != nil {
		t.Fatalf("schema: %v\n%s", err, stderr)
	}
	first := readFile(t, agentsPath(dir))
	for _, want := range []string{
		"gnosis:begin vocabulary", "gnosis:end vocabulary",
		"gnosis:begin commands", "gnosis:end commands",
		// The command list is the binary describing itself, so a command that
		// exists must appear.
		"gnosis fetch", "gnosis lint",
	} {
		if !strings.Contains(first, want) {
			t.Errorf("the written file lacks %q:\n%s", want, first)
		}
	}

	// A second run changes nothing, which is what `--check` rests on.
	if _, stderr, err := run(t, "--bundle", dir, "schema"); err != nil {
		t.Fatalf("second schema: %v\n%s", err, stderr)
	}
	if second := readFile(t, agentsPath(dir)); second != first {
		t.Errorf("a second run changed the file:\n%q\n%q", first, second)
	}
}

// TestCheckReportsStalenessWithoutWriting is §17's distinction: the examination
// completed and found something, which is a finding rather than a tool failure.
func TestCheckReportsStalenessWithoutWriting(t *testing.T) {
	t.Parallel()
	dir := corpus(t)

	if _, stderr, err := run(t, "--bundle", dir, "schema"); err != nil {
		t.Fatalf("schema: %v\n%s", err, stderr)
	}
	// Up to date: --check passes and writes nothing.
	if _, stderr, err := run(t, "--bundle", dir, "schema", "--check"); err != nil {
		t.Fatalf("--check on a fresh file: %v\n%s", err, stderr)
	}

	// Now make it stale by hand, the way a merge conflict or an old commit would.
	fresh := readFile(t, agentsPath(dir))
	stale := strings.Replace(fresh, "- `gnosis fetch`", "- `gnosis fetsh`", 1)
	if stale == fresh {
		t.Fatal("the fixture could not make the file stale")
	}
	if err := os.WriteFile(agentsPath(dir), []byte(stale), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, stderr, err := run(t, "--bundle", dir, "schema", "--check")
	if err == nil {
		t.Error("a stale file passed --check")
	}
	if !strings.Contains(stderr, "stale") {
		t.Errorf("the report does not say it is stale: %s", stderr)
	}
	// And --check wrote nothing, which is the whole of its contract.
	if got := readFile(t, agentsPath(dir)); got != stale {
		t.Error("--check modified the file")
	}
}

// TestAPersonsProseSurvivesAnUpdate is the region split the second backlog entry
// asked for, end to end: a refresh rewrites what the machine wrote and leaves what a
// person wrote alone.
func TestAPersonsProseSurvivesAnUpdate(t *testing.T) {
	t.Parallel()
	dir := corpus(t)

	if _, stderr, err := run(t, "--bundle", dir, "schema"); err != nil {
		t.Fatalf("schema: %v\n%s", err, stderr)
	}
	const note = "\n## Ask Priya first\n\nThe queue page is wrong and she knows why.\t\n"
	written := readFile(t, agentsPath(dir))
	if err := os.WriteFile(agentsPath(dir), []byte(written+note), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Make a generated region stale, so the next run has work to do inside it.
	edited := strings.Replace(readFile(t, agentsPath(dir)),
		"- `gnosis fetch`", "- `gnosis stale-entry`", 1)
	if err := os.WriteFile(agentsPath(dir), []byte(edited), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, stderr, err := run(t, "--bundle", dir, "schema"); err != nil {
		t.Fatalf("second schema: %v\n%s", err, stderr)
	}
	got := readFile(t, agentsPath(dir))
	if !strings.HasSuffix(got, note) {
		t.Errorf("the person's prose was changed or moved:\n%q", got)
	}
	if strings.Contains(got, "gnosis stale-entry") {
		t.Error("the stale generated region was not refreshed")
	}
	if !strings.Contains(got, "- `gnosis fetch`") {
		t.Error("the refreshed region does not carry the real command list")
	}
}

// TestLinkPointsEveryAgentFileAtOne is §5.7's borrowed rule: one canonical file, so
// several agents read it and cannot drift.
func TestLinkPointsEveryAgentFileAtOne(t *testing.T) {
	t.Parallel()
	dir := corpus(t)

	if _, stderr, err := run(t, "--bundle", dir, "schema"); err != nil {
		t.Fatalf("schema: %v\n%s", err, stderr)
	}
	if _, stderr, err := run(t, "--bundle", dir, "schema", "link"); err != nil {
		t.Fatalf("link: %v\n%s", err, stderr)
	}
	for _, name := range []string{"CLAUDE.md", "GEMINI.md", "QWEN.md"} {
		info, err := os.Lstat(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("%s was not created: %v", name, err)
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s is not a symlink", name)
		}
	}
	// Running it twice is not an error: repointing a symlink is what the command is
	// for, and a second run that refused would make the command unusable in a script.
	if _, stderr, err := run(t, "--bundle", dir, "schema", "link"); err != nil {
		t.Errorf("a second link failed: %v\n%s", err, stderr)
	}
}

// TestLinkRefusesToDeleteSomebodysFile is the data loss no flag should cause quietly.
func TestLinkRefusesToDeleteSomebodysFile(t *testing.T) {
	t.Parallel()
	dir := corpus(t)

	if _, stderr, err := run(t, "--bundle", dir, "schema"); err != nil {
		t.Fatalf("schema: %v\n%s", err, stderr)
	}
	const mine = "# Claude\n\nMy own instructions, written by hand.\n"
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(mine), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, stderr, err := run(t, "--bundle", dir, "schema", "link")
	if err == nil {
		t.Fatal("link replaced a hand-written file")
	}
	if got := readFile(t, filepath.Join(dir, "CLAUDE.md")); got != mine {
		t.Errorf("the hand-written file was modified:\n%q", got)
	}
	if !strings.Contains(stderr, "move it aside") {
		t.Errorf("the refusal does not say what to do: %s", stderr)
	}
}

// TestLinkNeedsSomethingToLinkTo keeps it from creating a bundle full of broken links.
//
// `init` now generates the schema document, so the way to reach this state is to remove
// it — which is the real case anyway: a bundle whose `AGENTS.md` somebody deleted, or
// one cloned from before the command existed.
func TestLinkNeedsSomethingToLinkTo(t *testing.T) {
	t.Parallel()
	dir := corpus(t)
	if err := os.Remove(agentsPath(dir)); err != nil {
		t.Fatalf("remove the generated schema document: %v", err)
	}

	_, stderr, err := run(t, "--bundle", dir, "schema", "link")
	if err == nil {
		t.Fatal("link succeeded with no AGENTS.md")
	}
	if !strings.Contains(stderr, "run `gnosis schema` first") {
		t.Errorf("the refusal does not say what to do: %s", stderr)
	}
}

// TestIndexKeepsCuratedProseAndRefreshesTheListing is the marker contract on the second
// document that adopts it, end to end.
//
// It matters more here than for AGENTS.md: index.md is where a person writes the
// handful of paths a newcomer needs, so losing that prose to a regeneration would take
// the one part of the file nothing else can reproduce.
func TestIndexKeepsCuratedProseAndRefreshesTheListing(t *testing.T) {
	t.Parallel()
	dir := corpus(t)

	const mine = "\n## Start here\n\nRead the retry budget page first.\t\n"
	indexPath := filepath.Join(dir, "index.md")
	before := readFile(t, indexPath)
	if !strings.Contains(before, "gnosis:begin index") {
		t.Fatalf("init did not generate the index region:\n%s", before)
	}
	if err := os.WriteFile(indexPath, []byte(before+mine), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, stderr, err := run(t, "--bundle", dir, "schema"); err != nil {
		t.Fatalf("schema: %v\n%s", err, stderr)
	}
	got := readFile(t, indexPath)
	if !strings.HasSuffix(got, mine) {
		t.Errorf("the curated prose was changed or moved:\n%q", got)
	}
	// corpus(t) writes two documents; both must be listed, with their titles.
	for _, want := range []string{"Retry budget", "Timeout policy", "**Reference**"} {
		if !strings.Contains(got, want) {
			t.Errorf("the listing lacks %q:\n%s", want, got)
		}
	}
}

// TestAFreshBundleIsQuietUnderSchema is the day-one property, and the reason init
// generates index.md rather than seeding prose.
//
// Seeding it unmarked made every `gnosis schema` on every bundle report a finding and
// exit non-zero from the moment the corpus was created — a signal firing hardest when
// there is least to say, which is the shape this project has now recorded four times.
func TestAFreshBundleIsQuietUnderSchema(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, _, err := run(t, "--bundle", dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	if _, stderr, err := run(t, "--bundle", dir, "schema"); err != nil {
		t.Fatalf("a freshly initialised bundle was not quiet: %v\n%s", err, stderr)
	}
	if _, stderr, err := run(t, "--bundle", dir, "schema", "--check"); err != nil {
		t.Fatalf("--check on a fresh bundle reported work: %v\n%s", err, stderr)
	}
	// And the empty listing says so rather than being blank.
	if got := readFile(
		t,
		filepath.Join(dir, "index.md"),
	); !strings.Contains(
		got,
		"no documents yet",
	) {
		t.Errorf("the empty listing does not say it is empty:\n%s", got)
	}
}

// TestAHandWrittenIndexIsNeverConverted is §5.7.1's fail-closed rule on the file where
// it is the common case rather than the edge: every bundle created before index.md was
// generated has a hand-written one.
func TestAHandWrittenIndexIsNeverConverted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, _, err := run(t, "--bundle", dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}

	const mine = "# Index\n\nMy own map, written before markers existed.   \n"
	indexPath := filepath.Join(dir, "index.md")
	if err := os.WriteFile(indexPath, []byte(mine), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, stderr, err := run(t, "--bundle", dir, "schema")
	if err == nil {
		t.Error("an unmarked index.md was reported as nothing to do")
	}
	// Byte for byte, including the trailing spaces a formatter would remove.
	if got := readFile(t, indexPath); got != mine {
		t.Errorf("the hand-written file changed:\n%q", got)
	}
	sibling := readFile(t, filepath.Join(dir, "index.generated.md"))
	if !strings.Contains(sibling, "gnosis:begin index") {
		t.Errorf("the generated listing was not written beside it:\n%s", sibling)
	}
	if !strings.Contains(stderr, "index.md carries no gnosis markers") {
		t.Errorf("the report does not name the file: %s", stderr)
	}
}
