// Package docs renders a deterministic team-index Markdown page for the
// pt-ekklesia-docs Docusaurus site.
//
// Output is the minimum stub humans extend with Bounded Context,
// Ubiquitous Language, ADRs, and so on. The renderer never overwrites
// hand-authored content — open_team_docs_pr is only meant to land the
// initial page; subsequent edits stay human-driven.
//
// Frontmatter is fixed at sidebar_label and description. Body is a
// heading, the description as a lede paragraph, and a placeholder
// Repositories section.
package docs

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

// Result is the rendered page plus its repo-relative path.
type Result struct {
	Path    string
	Content []byte
}

// Render produces the team's docs index page. The path is derived from
// team_type (section folder) and team_key (team folder).
//
// Returns an error rather than producing a degenerate page when:
//
//   - team is nil, team_key, display_name, or team_type is empty
//   - display_name_comment is empty (used as the frontmatter description
//     and the lede paragraph; without it the page is meaningless)
//   - team_type is not one of the four known Team Topologies values
func Render(t *spec.Team) (*Result, error) {
	if t == nil {
		return nil, fmt.Errorf("render: team is required")
	}
	if t.TeamKey == "" {
		return nil, fmt.Errorf("render: team_key is required")
	}
	if t.DisplayName == "" {
		return nil, fmt.Errorf("render: display_name is required")
	}
	if t.DisplayNameComment == "" {
		return nil, fmt.Errorf("render: display_name_comment is required (used as the docs page description)")
	}
	section, err := SectionFor(t.TeamType)
	if err != nil {
		return nil, err
	}
	folder, err := TeamFolder(t.TeamKey)
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer
	b.WriteString("---\n")
	b.WriteString("sidebar_label: ")
	b.WriteString(t.DisplayName)
	b.WriteString("\n")
	b.WriteString("description: ")
	b.WriteString(t.DisplayNameComment)
	b.WriteString("\n")
	b.WriteString("---\n\n")
	b.WriteString("# ")
	b.WriteString(t.DisplayName)
	b.WriteString("\n\n")
	b.WriteString(t.DisplayNameComment)
	b.WriteString("\n\n")
	b.WriteString("## Repositories\n\n")
	b.WriteString("<!-- Maintained by humans. List the team's repositories here. -->\n")

	return &Result{
		Path:    "docs/" + section + "/" + folder + "/index.md",
		Content: b.Bytes(),
	}, nil
}

// SectionFor maps a Team Topologies team_type to the Docusaurus section
// folder used by pt-ekklesia-docs.
func SectionFor(teamType string) (string, error) {
	switch teamType {
	case "platform-team":
		return "platform-grouping", nil
	case "stream-aligned-team":
		return "stream-aligned-teams", nil
	case "complicated-subsystem-team":
		return "complicated-subsystem-teams", nil
	case "enabling-team":
		return "enabling-teams", nil
	default:
		return "", fmt.Errorf("render: unknown team_type %q", teamType)
	}
}

// TeamFolder strips the team_type prefix (pt-/st-/ct-/et-) from team_key
// to derive the docs folder name. Matches the existing pt-ekklesia-docs
// layout where every team folder is the bare name (e.g. pt-logos -> logos).
func TeamFolder(teamKey string) (string, error) {
	for _, p := range []string{"pt-", "st-", "ct-", "et-"} {
		if strings.HasPrefix(teamKey, p) {
			return strings.TrimPrefix(teamKey, p), nil
		}
	}
	return "", fmt.Errorf("render: team_key %q does not start with pt-/st-/ct-/et-", teamKey)
}
