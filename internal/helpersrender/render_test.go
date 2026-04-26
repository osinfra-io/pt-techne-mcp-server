package helpersrender

import (
	"bytes"
	"strings"
	"testing"
)

// ptCorpusFixture mirrors osinfra-io/pt-corpus/helpers.tofu — sorted
// list, no trailing comma on the last element, two-space indentation.
const ptCorpusFixture = `# OpenTofu Core Helpers Module (osinfra.io)
# https://github.com/osinfra-io/pt-arche-core-helpers

module "core_helpers" {
  source = "github.com/osinfra-io/pt-arche-core-helpers//root?ref=70c1e4f4a11acdcfc36055bdd3aa16579d4203a1" # v0.3.1

  cost_center         = "x001"
  data_classification = "public"

  # Logos functionality - consolidated into core-helpers
  logos_workspaces = [
    "pt-corpus-main-production",
    "pt-kryptos-main-production",
    "pt-logos-main-production",
    "pt-pneuma-main-production",
    "pt-techne-main-production",
    "st-ethos-main-production"
  ]

  repository = "pt-corpus"
  team       = "pt-corpus"
}
`

// ptPneumaFixture mirrors osinfra-io/pt-pneuma/helpers.tofu — sorted
// list with a trailing comma on the last element.
const ptPneumaFixture = `module "core_helpers" {
  source = "github.com/osinfra-io/pt-arche-core-helpers//root?ref=70c1e4f4a11acdcfc36055bdd3aa16579d4203a1" # v0.3.1

  logos_workspaces = [
    "pt-corpus-main-production",
    "pt-kryptos-main-production",
    "pt-logos-main-production",
    "pt-pneuma-main-production",
  ]

  repository = "pt-pneuma"
  team       = "pt-pneuma"
}
`

func TestRender_InsertSorted_NoTrailingComma(t *testing.T) {
	out, err := Render([]byte(ptCorpusFixture), "pt-newteam-main-production")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := strings.Replace(
		ptCorpusFixture,
		`    "pt-pneuma-main-production",`,
		"    \"pt-newteam-main-production\",\n    \"pt-pneuma-main-production\",",
		1,
	)
	if string(out) != want {
		t.Fatalf("output mismatch\n--- got ---\n%s\n--- want ---\n%s", out, want)
	}
}

func TestRender_InsertSorted_TrailingComma(t *testing.T) {
	out, err := Render([]byte(ptPneumaFixture), "pt-newteam-main-production")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := strings.Replace(
		ptPneumaFixture,
		`    "pt-pneuma-main-production",`,
		"    \"pt-newteam-main-production\",\n    \"pt-pneuma-main-production\",",
		1,
	)
	if string(out) != want {
		t.Fatalf("output mismatch\n--- got ---\n%s\n--- want ---\n%s", out, want)
	}
}

func TestRender_AppendAtEnd_NoTrailingComma(t *testing.T) {
	// "zz-..." sorts after every existing entry → must be appended at
	// the end, and the previous last element gains a comma.
	out, err := Render([]byte(ptCorpusFixture), "zz-newteam-main-production")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := strings.Replace(
		ptCorpusFixture,
		"    \"st-ethos-main-production\"\n  ]",
		"    \"st-ethos-main-production\",\n    \"zz-newteam-main-production\"\n  ]",
		1,
	)
	if string(out) != want {
		t.Fatalf("output mismatch\n--- got ---\n%s\n--- want ---\n%s", out, want)
	}
}

func TestRender_AppendAtEnd_TrailingComma(t *testing.T) {
	out, err := Render([]byte(ptPneumaFixture), "zz-newteam-main-production")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := strings.Replace(
		ptPneumaFixture,
		"    \"pt-pneuma-main-production\",\n  ]",
		"    \"pt-pneuma-main-production\",\n    \"zz-newteam-main-production\",\n  ]",
		1,
	)
	if string(out) != want {
		t.Fatalf("output mismatch\n--- got ---\n%s\n--- want ---\n%s", out, want)
	}
}

func TestRender_Idempotent_BytesIdentical(t *testing.T) {
	for name, src := range map[string]string{
		"pt-corpus": ptCorpusFixture,
		"pt-pneuma": ptPneumaFixture,
	} {
		t.Run(name, func(t *testing.T) {
			in := []byte(src)
			out, err := Render(in, "pt-logos-main-production")
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if !bytes.Equal(in, out) {
				t.Fatalf("noop must return byte-identical output\n--- in ---\n%s\n--- out ---\n%s", in, out)
			}
		})
	}
}

func TestRender_AppendWhenUnsorted(t *testing.T) {
	src := `module "core_helpers" {
  logos_workspaces = [
    "pt-zeta-main-production",
    "pt-alpha-main-production",
  ]
}
`
	out, err := Render([]byte(src), "pt-mid-main-production")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	// "pt-mid-..." would land between alpha and zeta if sorted — but
	// the existing list is unsorted, so we append at the end instead.
	want := `module "core_helpers" {
  logos_workspaces = [
    "pt-zeta-main-production",
    "pt-alpha-main-production",
    "pt-mid-main-production",
  ]
}
`
	if string(out) != want {
		t.Fatalf("output mismatch\n--- got ---\n%s\n--- want ---\n%s", out, want)
	}
}

func TestRender_PreservesCRLF(t *testing.T) {
	src := strings.ReplaceAll(ptPneumaFixture, "\n", "\r\n")
	out, err := Render([]byte(src), "pt-newteam-main-production")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !bytes.Contains(out, []byte("\r\n    \"pt-newteam-main-production\",\r\n    \"pt-pneuma-main-production\",\r\n")) {
		t.Fatalf("CRLF not preserved around inserted line:\n%s", out)
	}
	if bytes.Contains(out, []byte("    \"pt-newteam-main-production\",\n    ")) {
		t.Fatalf("LF leaked into a CRLF input:\n%s", out)
	}
}

func TestRender_Errors(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		wantSub string
	}{
		{
			name:    "missing module",
			src:     `foo = "bar"` + "\n",
			wantSub: `module "core_helpers" block not found`,
		},
		{
			name: "duplicate module",
			src: `module "core_helpers" { logos_workspaces = ["a"] }
module "core_helpers" { logos_workspaces = ["b"] }
`,
			wantSub: `multiple module "core_helpers" blocks`,
		},
		{
			name: "missing attribute",
			src: `module "core_helpers" {
  source = "x"
}
`,
			wantSub: `logos_workspaces attribute not found`,
		},
		{
			name: "non-list value",
			src: `module "core_helpers" {
  logos_workspaces = "not-a-list"
}
`,
			wantSub: `must be a list literal`,
		},
		{
			name: "non-string element",
			src: `module "core_helpers" {
  logos_workspaces = ["ok", 42]
}
`,
			wantSub: `must be a plain string literal`,
		},
		{
			name: "interpolation element",
			src: `module "core_helpers" {
  prefix           = "pt"
  logos_workspaces = ["${prefix}-corpus-main-production"]
}
`,
			wantSub: `must be a plain string literal`,
		},
		{
			name: "empty list",
			src: `module "core_helpers" {
  logos_workspaces = []
}
`,
			wantSub: `cannot infer style for insertion`,
		},
		{
			name: "inline list",
			src: `module "core_helpers" {
  logos_workspaces = ["pt-corpus-main-production", "pt-logos-main-production"]
}
`,
			wantSub: `one element per line`,
		},
		{
			name:    "empty workspace input",
			src:     ptCorpusFixture,
			wantSub: `workspace is empty`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := "pt-new-main-production"
			if tc.name == "empty workspace input" {
				ws = ""
			}
			_, err := Render([]byte(tc.src), ws)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}
