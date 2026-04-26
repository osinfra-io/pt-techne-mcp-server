// Command schemadoc renders schema/team.schema.json into docs/schema.md.
//
// Output is intentionally minimal — one section per top-level field with
// type, required flag, description, and constraints (pattern/enum). For
// nested objects we link to the field by JSON Pointer. Run via:
//
//	make schema-docs
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type schema struct {
	Description          string             `json:"description"`
	Type                 any                `json:"type"`
	Required             []string           `json:"required"`
	Properties           map[string]*schema `json:"properties"`
	Items                *schema            `json:"items"`
	AdditionalProperties json.RawMessage    `json:"additionalProperties"`
	Enum                 []any              `json:"enum"`
	Pattern              string             `json:"pattern"`
	Defs                 map[string]*schema `json:"$defs"`
	Ref                  string             `json:"$ref"`
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: schemadoc <schema.json> <out.md>")
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		die(err)
	}
	var root schema
	if err := json.Unmarshal(data, &root); err != nil {
		die(err)
	}

	var b strings.Builder
	b.WriteString("# Team spec schema\n\n")
	b.WriteString("Generated from `schema/team.schema.json` — do not edit by hand.\n\n")
	b.WriteString("Run `make schema-docs` to regenerate.\n\n")

	teamDef := resolveTeamDef(&root)
	if teamDef == nil {
		die(fmt.Errorf("schema does not declare a team definition"))
	}
	required := setOf(teamDef.Required)

	for _, k := range sortedKeys(teamDef.Properties) {
		writeField(&b, k, teamDef.Properties[k], required[k], 2)
	}

	if err := os.WriteFile(os.Args[2], []byte(b.String()), 0o644); err != nil {
		die(err)
	}
}

func resolveTeamDef(root *schema) *schema {
	// Schema may be a top-level Team object (current shape) or a wrapper
	// `{ teams: { additionalProperties: <team> } }`. Handle both.
	if teams, ok := root.Properties["teams"]; ok && len(teams.AdditionalProperties) > 0 {
		var t schema
		if err := json.Unmarshal(teams.AdditionalProperties, &t); err != nil {
			return nil
		}
		if t.Ref != "" && root.Defs != nil {
			name := strings.TrimPrefix(t.Ref, "#/$defs/")
			if def, ok := root.Defs[name]; ok {
				return def
			}
		}
		return &t
	}
	return root
}

func writeField(b *strings.Builder, name string, s *schema, required bool, depth int) {
	prefix := strings.Repeat("#", depth)
	fmt.Fprintf(b, "%s `%s`\n\n", prefix, name)
	if s.Description != "" {
		b.WriteString(s.Description + "\n\n")
	}
	fmt.Fprintf(b, "- **type:** `%s`\n", typeStr(s.Type))
	fmt.Fprintf(b, "- **required:** %v\n", required)
	if s.Pattern != "" {
		fmt.Fprintf(b, "- **pattern:** `%s`\n", s.Pattern)
	}
	if len(s.Enum) > 0 {
		parts := make([]string, len(s.Enum))
		for i, v := range s.Enum {
			parts[i] = fmt.Sprintf("`%v`", v)
		}
		fmt.Fprintf(b, "- **enum:** %s\n", strings.Join(parts, ", "))
	}
	b.WriteString("\n")
}

func typeStr(t any) string {
	switch v := t.(type) {
	case string:
		return v
	case []any:
		parts := make([]string, len(v))
		for i, x := range v {
			parts[i] = fmt.Sprintf("%v", x)
		}
		return strings.Join(parts, " | ")
	}
	return "any"
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func setOf(ss []string) map[string]bool {
	out := make(map[string]bool, len(ss))
	for _, s := range ss {
		out[s] = true
	}
	return out
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "schemadoc:", err)
	os.Exit(1)
}
