// teamKeyRe matches the same pattern as schema/team.schema.json's
// team_key. Used by tools that accept a team_key directly (e.g. the
// helpers renderers) and never call the validator.
package tools

import "regexp"

var teamKeyRe = regexp.MustCompile(`^(pt|st|ct|et)-[a-z][a-z0-9-]*[a-z0-9]$`)
