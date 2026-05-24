// Package sidebar inserts a team's category entry into the
// pt-ekklesia-docs sidebars.js file by locating // region:<section>
// markers and appending a category block just before the matching
// // endregion: marker.
//
// The renderer is intentionally text-based — sidebars.js is real JS, not
// data — and relies on the // region: <section> / // endregion: <section>
// anchor convention added by the prep PR. Missing anchors return a
// structured error instead of any silent fallback.
//
// Design notes:
//
//   - Existing entries are never reordered. The visible order in
//     sidebars.js conveys meaning (the platform team list is roughly the
//     dependency order Logos → Corpus → Pneuma → Arche → ...), and
//     alphabetising would silently rewrite it. The renderer therefore
//     appends new entries at the bottom of the region. Humans reorder
//     manually when they care about position.
//   - Output is byte-stable on noop: when the entry is already present
//     between the markers (in any form — plain string or category whose
//     link.id matches), the input bytes are returned unchanged.
package sidebar

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// ErrAnchorsMissing is returned when the // region: / // endregion:
// markers for a section are absent or malformed. Callers should surface
// this as source_parse_error.
type ErrAnchorsMissing struct {
	Section string
}

func (e *ErrAnchorsMissing) Error() string {
	return fmt.Sprintf("sidebars.js: missing or malformed // region: %s / // endregion: %s anchor pair", e.Section, e.Section)
}

// maxSidebarBytes bounds the input size accepted by Render. Real
// sidebars.js files are <100 KiB; this cap exists purely to make the
// allocation in Render obviously unable to overflow.
const maxSidebarBytes = 10 << 20 // 10 MiB

// Render inserts a category entry for the given team into the region for
// section. If the entry is already present (in any form — either as a
// plain string or as a category referencing the same id), the input bytes
// are returned unchanged.
//
// section is the Docusaurus section folder (e.g. "platform-grouping"),
// teamFolder is the team's folder name within that section (e.g. "logos"),
// and label is the human-readable sidebar label (e.g. "Logos").
// All must be non-empty.
func Render(existing []byte, section, teamFolder, label string) ([]byte, error) {
	if section == "" {
		return nil, fmt.Errorf("sidebar: section is required")
	}
	if teamFolder == "" {
		return nil, fmt.Errorf("sidebar: team_folder is required")
	}
	if label == "" {
		return nil, fmt.Errorf("sidebar: label is required")
	}
	if len(existing) > maxSidebarBytes {
		return nil, fmt.Errorf("sidebar: input is %d bytes, exceeds %d byte cap", len(existing), maxSidebarBytes)
	}

	regionStart, err := findRegionStart(existing, section)
	if err != nil {
		return nil, err
	}
	endLineStart, indent, err := findEndregion(existing, section)
	if err != nil {
		return nil, err
	}
	// A misordered anchor pair (endregion before region) would otherwise
	// panic on the slice below. Surface it as the same structured error
	// callers already handle for missing anchors.
	if endLineStart < regionStart {
		return nil, &ErrAnchorsMissing{Section: section}
	}

	id := section + "/" + teamFolder + "/index"
	region := existing[regionStart:endLineStart]
	if regionContainsID(region, id) {
		return existing, nil
	}

	// Emit a category block matching the established pattern in sidebars.js.
	// JSON-encode dynamic strings to prevent JS injection from labels
	// containing quotes or newlines.
	safeLabel, _ := json.Marshal(label)
	safeID, _ := json.Marshal(id)
	block := indent + "{\n" +
		indent + "  type: 'category',\n" +
		indent + "  label: " + string(safeLabel) + ",\n" +
		indent + "  link: { type: 'doc', id: " + string(safeID) + " },\n" +
		indent + "  items: [],\n" +
		indent + "},\n"

	// Build via append rather than make([]byte, 0, len(existing)+len(block)):
	// the explicit capacity computation tripped a CodeQL overflow heuristic
	// even though the input is bounded by maxSidebarBytes above.
	out := append([]byte(nil), existing[:endLineStart]...)
	out = append(out, []byte(block)...)
	out = append(out, existing[endLineStart:]...)
	return out, nil
}

// findRegionStart returns the byte offset immediately after the
// // region: <section> marker line.
func findRegionStart(src []byte, section string) (int, error) {
	re := regexp.MustCompile(`(?m)^[ \t]*// region: ` + regexp.QuoteMeta(section) + `[ \t]*\r?\n`)
	m := re.FindIndex(src)
	if m == nil {
		return 0, &ErrAnchorsMissing{Section: section}
	}
	return m[1], nil
}

// findEndregion returns the byte offset at the start of the
// // endregion: <section> marker line plus the indentation string used
// on that line. Insertions go at this offset so the new entry lands
// just before the marker, with matching indentation.
func findEndregion(src []byte, section string) (int, string, error) {
	re := regexp.MustCompile(`(?m)^([ \t]*)// endregion: ` + regexp.QuoteMeta(section) + `[ \t]*\r?\n`)
	m := re.FindSubmatchIndex(src)
	if m == nil {
		return 0, "", &ErrAnchorsMissing{Section: section}
	}
	return m[0], string(src[m[2]:m[3]]), nil
}

var (
	plainEntryRe = regexp.MustCompile(`(?m)^[ \t]*'([^']+)',?\s*$`)
	categoryIDRe = regexp.MustCompile(`id:\s*['"]([^'"]+)['"]`)
)

// regionContainsID reports whether the given region bytes already
// reference id, either as a plain string entry or inside a category's
// link.id.
func regionContainsID(region []byte, id string) bool {
	for _, m := range plainEntryRe.FindAllSubmatch(region, -1) {
		if string(m[1]) == id {
			return true
		}
	}
	for _, m := range categoryIDRe.FindAllSubmatch(region, -1) {
		if string(m[1]) == id {
			return true
		}
	}
	return false
}

// EnsureAnchors is a defensive helper a caller can use to verify a
// sidebars.js source has all expected section anchors before attempting
// any edit. Returns nil when every section in sections has a matching
// anchor pair; otherwise returns the first ErrAnchorsMissing.
func EnsureAnchors(src []byte, sections ...string) error {
	for _, s := range sections {
		if s == "" {
			return fmt.Errorf("sidebar: section is required")
		}
		regionStart, err := findRegionStart(src, s)
		if err != nil {
			return err
		}
		endLineStart, _, err := findEndregion(src, s)
		if err != nil {
			return err
		}
		if endLineStart < regionStart {
			return &ErrAnchorsMissing{Section: s}
		}
	}
	return nil
}
