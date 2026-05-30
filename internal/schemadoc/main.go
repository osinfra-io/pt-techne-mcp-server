// Command schemadoc renders schema/team.schema.json into docs/schema.md.
//
// Output is intentionally minimal — one section per top-level field with
// type, required flag, description, and constraints (pattern/enum). Nested
// objects are not expanded. Run via:
//
//	go run ./internal/schemadoc schema/team.schema.json docs/schema.md
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
	data, err := os.ReadFile(os.Args[1]) //nolint:gosec // dev-only CLI generator; path is its own argv, not untrusted input
	if err != nil {
		die(fmt.Errorf("read schema %q: %w", os.Args[1], err))
	}
	var root schema
	if err := json.Unmarshal(data, &root); err != nil {
		die(fmt.Errorf("parse schema %q: %w", os.Args[1], err))
	}

	var b strings.Builder
	b.WriteString("# Team spec schema\n\n")
	b.WriteString("Generated from `schema/team.schema.json` — do not edit by hand.\n\n")
	b.WriteString("Run `go run ./internal/schemadoc schema/team.schema.json docs/schema.md` to regenerate.\n\n")

	teamDef, err := resolveTeamDef(&root)
	if err != nil {
		die(err)
	}
	if teamDef == nil {
		die(fmt.Errorf("schema does not declare a team definition"))
	}
	required := setOf(teamDef.Required)

	for _, k := range sortedKeys(teamDef.Properties) {
		field := teamDef.Properties[k]
		if field == nil {
			die(fmt.Errorf("render field %q: schema is null", k))
		}
		writeField(&b, k, field, required[k], 2, root.Defs)
	}

	out := strings.TrimRight(b.String(), "\n") + "\n"

	if err := os.WriteFile(os.Args[2], []byte(out), 0o600); err != nil { //nolint:gosec // dev-only CLI generator; path is its own argv, not untrusted input
		die(fmt.Errorf("write markdown %q: %w", os.Args[2], err))
	}
}

func resolveTeamDef(root *schema) (*schema, error) {
	// Schema may be a top-level Team object (current shape) or a wrapper
	// `{ teams: { additionalProperties: <team> } }`. Handle both.
	teams, ok := root.Properties["teams"]
	if ok && teams != nil && len(teams.AdditionalProperties) > 0 {
		var t schema
		if err := json.Unmarshal(teams.AdditionalProperties, &t); err != nil {
			return nil, fmt.Errorf("unmarshal teams.additionalProperties: %w", err)
		}
		if t.Ref != "" {
			if root.Defs == nil {
				return nil, fmt.Errorf("resolve %q: schema has no $defs", t.Ref)
			}
			name := strings.TrimPrefix(t.Ref, "#/$defs/")
			def, ok := root.Defs[name]
			if !ok {
				return nil, fmt.Errorf("resolve %q: definition %q not found", t.Ref, name)
			}
			return def, nil
		}
		return &t, nil
	}
	return root, nil
}

func writeField(b *strings.Builder, name string, s *schema, required bool, depth int, defs map[string]*schema) {
	// Resolve $ref before rendering. Deep-copy the resolved schema to
	// avoid mutating the shared definition.
	if s.Ref != "" && defs != nil {
		refName := strings.TrimPrefix(s.Ref, "#/$defs/")
		if resolved, ok := defs[refName]; ok {
			cp := *resolved
			// Preserve the parent description if the $ref target lacks one.
			if s.Description != "" && cp.Description == "" {
				cp.Description = s.Description
			}
			s = &cp
		}
	}

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
