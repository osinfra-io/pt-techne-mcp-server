package sidebar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixture = `// @ts-check

const sidebars = {
  docs: [
    {
      type: 'category',
      label: 'Platform Teams',
      items: [
        // region: platform-teams
        {
          type: 'category',
          label: 'Logos',
          link: { type: 'doc', id: 'platform-teams/logos/index' },
          items: [
            'platform-teams/logos/resource-hierarchy',
          ],
        },
        // endregion: platform-teams
      ],
    },
    {
      type: 'category',
      label: 'Stream-Aligned Teams',
      items: [
        // region: stream-aligned-teams
        // endregion: stream-aligned-teams
      ],
    },
  ],
};

export default sidebars;
`

func TestRenderInsertIntoEmptyRegion(t *testing.T) {
	got, err := Render([]byte(fixture), "stream-aligned-teams", "ethos")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(got), "'stream-aligned-teams/ethos/index',\n") {
		t.Fatalf("expected new entry; got:\n%s", got)
	}
	// New line must sit just before the endregion marker, indented to match.
	want := "        'stream-aligned-teams/ethos/index',\n        // endregion: stream-aligned-teams"
	if !strings.Contains(string(got), want) {
		t.Errorf("entry not placed before endregion with matching indent; got:\n%s", got)
	}
}

func TestRenderInsertIntoPopulatedRegion(t *testing.T) {
	got, err := Render([]byte(fixture), "platform-teams", "corpus")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// Existing Logos entry must be unchanged and untouched.
	if !strings.Contains(string(got), "label: 'Logos',\n") {
		t.Errorf("existing Logos entry was disturbed; got:\n%s", got)
	}
	if !strings.Contains(string(got), "        'platform-teams/corpus/index',\n        // endregion: platform-teams") {
		t.Errorf("new entry not placed at end of region; got:\n%s", got)
	}
}

func TestRenderNoopWhenPlainEntryAlreadyPresent(t *testing.T) {
	src := []byte(`// @ts-check
const sidebars = {
  docs: [{ items: [
    // region: platform-teams
    'platform-teams/logos/index',
    // endregion: platform-teams
  ]}],
};
`)
	got, err := Render(src, "platform-teams", "logos")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if string(got) != string(src) {
		t.Errorf("noop expected; got:\n%s\nwant:\n%s", got, src)
	}
}

func TestRenderNoopWhenCategoryAlreadyReferencesID(t *testing.T) {
	got, err := Render([]byte(fixture), "platform-teams", "logos")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if string(got) != fixture {
		t.Errorf("noop expected when category already references id; output diverged")
	}
}

func TestRenderMisorderedAnchors(t *testing.T) {
	// endregion appears before region — must surface ErrAnchorsMissing
	// instead of panicking on the slice.
	src := []byte(`// @ts-check
const sidebars = { docs: [{ items: [
  // endregion: platform-teams
  // region: platform-teams
] }] };
`)
	_, err := Render(src, "platform-teams", "foo")
	if err == nil {
		t.Fatalf("expected ErrAnchorsMissing, got nil")
	}
	var anchorsErr *ErrAnchorsMissing
	if !errAs(err, &anchorsErr) {
		t.Errorf("error type: got %T, want *ErrAnchorsMissing", err)
	}
}

func TestRenderMissingAnchors(t *testing.T) {
	src := []byte(`// @ts-check
const sidebars = { docs: [{ items: [] }] };
`)
	_, err := Render(src, "platform-teams", "foo")
	if err == nil {
		t.Fatalf("expected ErrAnchorsMissing, got nil")
	}
	var anchorsErr *ErrAnchorsMissing
	if !errAs(err, &anchorsErr) {
		t.Errorf("error type: got %T, want *ErrAnchorsMissing", err)
	}
}

func TestRenderInputValidation(t *testing.T) {
	if _, err := Render([]byte(fixture), "", "foo"); err == nil {
		t.Errorf("empty section should fail")
	}
	if _, err := Render([]byte(fixture), "platform-teams", ""); err == nil {
		t.Errorf("empty teamFolder should fail")
	}
}

func TestEnsureAnchors(t *testing.T) {
	if err := EnsureAnchors([]byte(fixture), "platform-teams", "stream-aligned-teams"); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if err := EnsureAnchors([]byte(fixture), "missing-section"); err == nil {
		t.Errorf("expected ErrAnchorsMissing for absent section")
	}
	misordered := []byte(`// @ts-check
const sidebars = { docs: [{ items: [
  // endregion: platform-teams
  // region: platform-teams
] }] };
`)
	err := EnsureAnchors(misordered, "platform-teams")
	if err == nil {
		t.Fatalf("expected ErrAnchorsMissing for misordered anchors, got nil")
	}
	var anchorsErr *ErrAnchorsMissing
	if !errAs(err, &anchorsErr) {
		t.Errorf("misordered anchors: error type = %T, want *ErrAnchorsMissing", err)
	}
}

// errAs is a tiny errors.As shim to keep the test file's imports tight.
func errAs(err error, target **ErrAnchorsMissing) bool {
	if e, ok := err.(*ErrAnchorsMissing); ok {
		*target = e
		return true
	}
	return false
}

// TestRenderAgainstRealFixture exercises the real pt-ekklesia-docs
// sidebars.js (post anchor-comment prep PR) to catch any drift between
// the renderer and the file shape it must edit.
func TestRenderAgainstRealFixture(t *testing.T) {
	path := filepath.Join("testdata", "sidebars.js")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("real fixture missing at %s: %v", path, err)
	}
	for _, section := range []string{"platform-teams", "stream-aligned-teams", "complicated-subsystem-teams", "enabling-teams"} {
		out, err := Render(src, section, "newteam")
		if err != nil {
			t.Errorf("render %s: %v", section, err)
			continue
		}
		want := "'" + section + "/newteam/index',"
		if !strings.Contains(string(out), want) {
			t.Errorf("section %s: expected %q in output", section, want)
		}
	}
}
