package splitcmd

import "testing"

func TestLocate(t *testing.T) {
	const (
		sources   = "/kb/source_materials"
		originals = "/kb/split_source_originals"
	)
	cases := map[string]struct {
		path, sources, originals string
		want                     placement
		refused                  bool
	}{
		"no --originals leaves the source beside its parts": {
			path: "/kb/source_materials/kleppmann/ddia.md", sources: sources,
			want: placement{partsDir: "/kb/source_materials/kleppmann"},
		},
		"a source moves to the same place under --originals": {
			path: "/kb/source_materials/kleppmann/ddia.md", sources: sources, originals: originals,
			want: placement{
				partsDir: "/kb/source_materials/kleppmann",
				moveTo:   "/kb/split_source_originals/kleppmann/ddia.md",
			},
		},
		"a moved original is split back into --sources and stays": {
			path: "/kb/split_source_originals/kleppmann/ddia.md", sources: sources,
			originals: originals,
			want:      placement{partsDir: "/kb/source_materials/kleppmann"},
		},
		"--originals inside --sources is still found first": {
			path: "/kb/split_source_originals/kleppmann/ddia.md", sources: "/kb",
			originals: originals,
			want:      placement{partsDir: "/kb/kleppmann"},
		},
		"a sibling directory sharing a prefix is not inside": {
			path: "/kb/source_materials_old/ddia.md", sources: sources, originals: originals,
			refused: true,
		},
		"a path under neither root is refused": {
			path: "/elsewhere/ddia.md", sources: sources, originals: originals,
			refused: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, refused := locate(tc.path, tc.sources, tc.originals)
			if (refused != "") != tc.refused {
				t.Fatalf("refused = %q, want a refusal %v", refused, tc.refused)
			}
			if got != tc.want {
				t.Errorf("placement = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestDecideMove(t *testing.T) {
	original := []byte("# Book\n")
	cases := map[string]struct {
		existing  []byte
		exists    bool
		conflicts int
		want      string
	}{
		"nothing there":          {want: originalMoved},
		"the same bytes already": {existing: original, exists: true, want: originalAlready},
		"other bytes there": {
			existing: []byte("# Other\n"),
			exists:   true,
			want:     originalConflict,
		},
		"a part is in conflict": {conflicts: 1, want: originalKept},
		"conflict outranks the same": {
			existing:  original,
			exists:    true,
			conflicts: 2,
			want:      originalKept,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := decideMove(original, tc.existing, tc.exists, tc.conflicts); got != tc.want {
				t.Errorf("decideMove = %q, want %q", got, tc.want)
			}
		})
	}
}
