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
      label: 'Platform Grouping',
      items: [
        // region: platform-grouping
        {
          type: 'category',
          label: 'Logos',
          link: { type: 'doc', id: 'platform-grouping/logos/index' },
          items: [
            'platform-grouping/logos/resource-hierarchy',
          ],
        },
        // endregion: platform-grouping
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
	got, err := Render([]byte(fixture), "stream-aligned-teams", "ethos", "Ethos")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(got), `id: "stream-aligned-teams/ethos/index"`) {
		t.Fatalf("expected new category entry; got:\n%s", got)
	}
	// New block must sit just before the endregion marker, indented to match.
	want := "        },\n        // endregion: stream-aligned-teams"
	if !strings.Contains(string(got), want) {
		t.Errorf("category not placed before endregion with matching indent; got:\n%s", got)
	}
	// Verify the category structure
	if !strings.Contains(string(got), `label: "Ethos",`) {
		t.Errorf("expected label 'Ethos'; got:\n%s", got)
	}
	if !strings.Contains(string(got), "items: [],") {
		t.Errorf("expected empty items; got:\n%s", got)
	}
}

func TestRenderInsertIntoPopulatedRegion(t *testing.T) {
	got, err := Render([]byte(fixture), "platform-grouping", "corpus", "Corpus")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// Existing Logos entry must be unchanged and untouched.
	if !strings.Contains(string(got), "label: 'Logos',\n") {
		t.Errorf("existing Logos entry was disturbed; got:\n%s", got)
	}
	if !strings.Contains(string(got), `link: { type: 'doc', id: "platform-grouping/corpus/index" },`+"\n") {
		t.Errorf("new category not placed in region; got:\n%s", got)
	}
}

func TestRenderNoopWhenPlainEntryAlreadyPresent(t *testing.T) {
	src := []byte(`// @ts-check
const sidebars = {
  docs: [{ items: [
    // region: platform-grouping
    'platform-grouping/logos/index',
    // endregion: platform-grouping
  ]}],
};
`)
	got, err := Render(src, "platform-grouping", "logos", "Logos")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if string(got) != string(src) {
		t.Errorf("noop expected; got:\n%s\nwant:\n%s", got, src)
	}
}

func TestRenderNoopWhenCategoryAlreadyReferencesID(t *testing.T) {
	got, err := Render([]byte(fixture), "platform-grouping", "logos", "Logos")
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
  // endregion: platform-grouping
  // region: platform-grouping
] }] };
`)
	_, err := Render(src, "platform-grouping", "foo", "Foo")
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
	_, err := Render(src, "platform-grouping", "foo", "Foo")
	if err == nil {
		t.Fatalf("expected ErrAnchorsMissing, got nil")
	}
	var anchorsErr *ErrAnchorsMissing
	if !errAs(err, &anchorsErr) {
		t.Errorf("error type: got %T, want *ErrAnchorsMissing", err)
	}
}

func TestRenderInputValidation(t *testing.T) {
	if _, err := Render([]byte(fixture), "", "foo", "Foo"); err == nil {
		t.Errorf("empty section should fail")
	}
	if _, err := Render([]byte(fixture), "platform-grouping", "", "Foo"); err == nil {
		t.Errorf("empty teamFolder should fail")
	}
	if _, err := Render([]byte(fixture), "platform-grouping", "foo", ""); err == nil {
		t.Errorf("empty label should fail")
	}
}

func TestEnsureAnchors(t *testing.T) {
	if err := EnsureAnchors([]byte(fixture), "platform-grouping", "stream-aligned-teams"); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if err := EnsureAnchors([]byte(fixture), ""); err == nil {
		t.Errorf("expected error for empty section name")
	}
	if err := EnsureAnchors([]byte(fixture), "missing-section"); err == nil {
		t.Errorf("expected ErrAnchorsMissing for absent section")
	}
	misordered := []byte(`// @ts-check
const sidebars = { docs: [{ items: [
  // endregion: platform-grouping
  // region: platform-grouping
] }] };
`)
	err := EnsureAnchors(misordered, "platform-grouping")
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
	for _, section := range []string{"platform-grouping", "stream-aligned-teams", "complicated-subsystem-teams", "enabling-teams"} {
		out, err := Render(src, section, "newteam", "Newteam")
		if err != nil {
			t.Errorf("render %s: %v", section, err)
			continue
		}
		want := `id: "` + section + `/newteam/index"`
		if !strings.Contains(string(out), want) {
			t.Errorf("section %s: expected %q in output", section, want)
		}
	}
}
