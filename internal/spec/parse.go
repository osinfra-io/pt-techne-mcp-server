// Parser: canonical pt-logos teams/<key>.tfvars  ->  *spec.Team.
//
// Strategy: parse the file with hclsyntax, evaluate the single
// top-level `teams` attribute to a cty value (the literal evaluates
// without an EvalContext because ObjectConsKeyExpr promotes bare
// identifiers to string keys), then marshal the value to JSON and
// strict-unmarshal into Team. Strictness (DisallowUnknownFields) means
// a human edit on pt-logos that introduces a misspelled or new field
// fails loudly here rather than silently dropping data.
//
// The HCL grammar drops comments, so display_name_comment is recovered
// from the raw bytes with a small regex. The regex only matches the
// renderer's exact line shape; if a human reformats that line the
// comment is dropped silently — that is an acceptable trade for
// keeping the parser narrow.
package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

var displayNameCommentRe = regexp.MustCompile(`(?m)^\s*display_name\s*=\s*"[^"]*"\s*#\s*(.*\S)\s*$`)

// Parse decodes canonical tfvars bytes into a Team. The input is
// expected to be the output of render.Render — exactly one team under
// the top-level teams attribute, with attributes that exactly match
// spec.Team's JSON tags. Inputs outside that shape return an error
// rather than a partial result.
func Parse(src []byte) (*Team, error) {
	f, diags := hclsyntax.ParseConfig(src, "team.tfvars", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse hcl: %s", diags.Error())
	}
	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		return nil, fmt.Errorf("parse hcl: unexpected body type %T", f.Body)
	}
	if len(body.Blocks) > 0 {
		return nil, fmt.Errorf("parse hcl: unexpected block %q at top level", body.Blocks[0].Type)
	}
	for name := range body.Attributes {
		if name != "teams" {
			return nil, fmt.Errorf("parse hcl: unexpected top-level attribute %q (only 'teams' is allowed)", name)
		}
	}
	attr, ok := body.Attributes["teams"]
	if !ok {
		return nil, fmt.Errorf("parse hcl: missing top-level teams attribute")
	}
	val, vdiags := attr.Expr.Value(nil)
	if vdiags.HasErrors() {
		return nil, fmt.Errorf("eval teams: %s", vdiags.Error())
	}
	t := val.Type()
	if !t.IsObjectType() && !t.IsMapType() {
		return nil, fmt.Errorf("teams: expected object, got %s", t.FriendlyName())
	}

	it := val.ElementIterator()
	if !it.Next() {
		return nil, fmt.Errorf("teams: empty (expected exactly one team)")
	}
	keyV, teamV := it.Element()
	if it.Next() {
		return nil, fmt.Errorf("teams: multiple entries (expected exactly one team)")
	}

	jsonBytes, err := ctyjson.Marshal(teamV, teamV.Type())
	if err != nil {
		return nil, fmt.Errorf("marshal team value: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(jsonBytes))
	dec.DisallowUnknownFields()
	var team Team
	if err := dec.Decode(&team); err != nil {
		return nil, fmt.Errorf("decode team value: %w", err)
	}
	outerKey := keyV.AsString()
	if team.TeamKey != "" && team.TeamKey != outerKey {
		return nil, fmt.Errorf("team_key mismatch: outer key %q != inner team_key %q", outerKey, team.TeamKey)
	}
	team.TeamKey = outerKey
	if m := displayNameCommentRe.FindSubmatch(src); m != nil {
		team.DisplayNameComment = string(m[1])
	}
	return &team, nil
}
