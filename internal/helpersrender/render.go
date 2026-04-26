// Package helpersrender inserts a workspace string into the
// logos_workspaces list inside a sibling repo's helpers.tofu without
// reformatting any other byte of the file.
//
// The renderer is byte-stable on noop: if the requested workspace is
// already present, the input bytes are returned unchanged. On insert,
// only the bytes for the new line are added; existing entries, comments,
// indentation, line-ending style, and trailing-comma style are
// preserved exactly.
//
// Used by render_corpus_helpers and render_pneuma_helpers, which fetch
// helpers.tofu from osinfra-io/pt-corpus@main and osinfra-io/pt-pneuma@main
// respectively.
package helpersrender

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// Render inserts workspace into the logos_workspaces list inside the
// module "core_helpers" block of helpers.tofu. The file must contain
// exactly one such module block and exactly one logos_workspaces
// attribute. The attribute must be a list literal of bare string
// literals; computed expressions, mixed types, and other shapes are
// rejected.
//
// If workspace is already present, the returned slice is byte-identical
// to existing. Otherwise one new line is inserted in alphabetical
// position relative to the existing entries (or appended at the end if
// the existing entries are not already sorted), matching the file's
// indentation, line-ending style, and trailing-comma convention.
func Render(existing []byte, workspace string) ([]byte, error) {
	if workspace == "" {
		return nil, fmt.Errorf("workspace is empty")
	}
	f, diags := hclsyntax.ParseConfig(existing, "helpers.tofu", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse hcl: %s", diags.Error())
	}
	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		return nil, fmt.Errorf("unexpected body type %T", f.Body)
	}

	mod, err := findCoreHelpersModule(body)
	if err != nil {
		return nil, err
	}
	attr, err := findLogosWorkspacesAttr(mod)
	if err != nil {
		return nil, err
	}
	tup, ok := attr.Expr.(*hclsyntax.TupleConsExpr)
	if !ok {
		return nil, fmt.Errorf(`logos_workspaces must be a list literal, got %T`, attr.Expr)
	}

	values, err := stringElements(tup)
	if err != nil {
		return nil, err
	}

	for _, v := range values {
		if v == workspace {
			// Idempotent noop: byte-identical return.
			return existing, nil
		}
	}

	if len(tup.Exprs) == 0 {
		return nil, fmt.Errorf("logos_workspaces is empty; cannot infer style for insertion")
	}

	eol := detectEOL(existing)
	indent := elementIndent(existing, tup.Exprs[0].Range())
	insertIdx := alphaInsertIndex(values, workspace)

	if insertIdx < len(tup.Exprs) {
		return insertBefore(existing, tup.Exprs[insertIdx].Range(), workspace, indent, eol), nil
	}
	return appendAfterLast(existing, tup, workspace, indent, eol)
}

// findCoreHelpersModule returns the body of the single
// module "core_helpers" block in body, erroring if there are zero or
// more than one.
func findCoreHelpersModule(body *hclsyntax.Body) (*hclsyntax.Body, error) {
	var found *hclsyntax.Block
	for _, b := range body.Blocks {
		if b.Type != "module" || len(b.Labels) != 1 || b.Labels[0] != "core_helpers" {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf(`multiple module "core_helpers" blocks found`)
		}
		found = b
	}
	if found == nil {
		return nil, fmt.Errorf(`module "core_helpers" block not found`)
	}
	return found.Body, nil
}

// findLogosWorkspacesAttr returns the single logos_workspaces attribute
// inside mod. The HCL parser already rejects duplicate attributes at
// parse time so we only need the missing case here.
func findLogosWorkspacesAttr(mod *hclsyntax.Body) (*hclsyntax.Attribute, error) {
	attr, ok := mod.Attributes["logos_workspaces"]
	if !ok {
		return nil, fmt.Errorf(`logos_workspaces attribute not found in module "core_helpers"`)
	}
	return attr, nil
}

// stringElements extracts every element of tup as a plain string
// literal, in source order. Anything other than a bare quoted string
// (e.g. a variable reference or interpolation) is rejected so the
// renderer never silently rewrites a computed list.
func stringElements(tup *hclsyntax.TupleConsExpr) ([]string, error) {
	out := make([]string, 0, len(tup.Exprs))
	for i, e := range tup.Exprs {
		te, ok := e.(*hclsyntax.TemplateExpr)
		if !ok || !te.IsStringLiteral() {
			return nil, fmt.Errorf("logos_workspaces[%d]: must be a plain string literal", i)
		}
		v, diags := te.Value(nil)
		if diags.HasErrors() {
			return nil, fmt.Errorf("logos_workspaces[%d]: %s", i, diags.Error())
		}
		if v.Type().FriendlyName() != "string" {
			return nil, fmt.Errorf("logos_workspaces[%d]: not a string", i)
		}
		out = append(out, v.AsString())
	}
	return out, nil
}

// alphaInsertIndex returns the index at which workspace should be
// inserted to keep values sorted, when values is already sorted. When
// values is not sorted, the returned index is len(values) so the new
// entry is appended rather than guessing where it would belong.
func alphaInsertIndex(values []string, workspace string) int {
	if !sort.StringsAreSorted(values) {
		return len(values)
	}
	return sort.SearchStrings(values, workspace)
}

// detectEOL returns "\r\n" if any CRLF line endings appear in src,
// otherwise "\n". helpers.tofu in the foundation repos uses LF; CRLF
// support is a defensive measure for inputs that came through a
// CRLF-converting transport.
func detectEOL(src []byte) string {
	if bytes.Contains(src, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}

// elementIndent returns the leading whitespace on the line that
// contains the element at r. Used to indent the new element identically
// to the existing entries.
func elementIndent(src []byte, r hcl.Range) string {
	start := r.Start.Byte
	if start <= 0 || start > len(src) {
		return ""
	}
	lineStart := bytes.LastIndexByte(src[:start], '\n') + 1
	prefix := src[lineStart:start]
	for i := 0; i < len(prefix); i++ {
		if prefix[i] != ' ' && prefix[i] != '\t' {
			return string(prefix[:i])
		}
	}
	return string(prefix)
}

// insertBefore splices a new line for workspace immediately before the
// element at r. The new line carries a trailing comma so the element
// after it remains formatted as it was.
func insertBefore(src []byte, r hcl.Range, workspace, indent, eol string) []byte {
	lineStart := bytes.LastIndexByte(src[:r.Start.Byte], '\n') + 1
	insertion := []byte(indent + `"` + workspace + `",` + eol)
	return spliceAt(src, lineStart, insertion)
}

// appendAfterLast inserts workspace as a new last element. Two cases:
//   - The list already has a trailing comma after the previous last
//     element: insert a fully-terminated line right after that comma's
//     line.
//   - The list does NOT have a trailing comma: insert
//     ",<eol><indent><quoted>" right after the previous last element,
//     which both adds the missing comma to the previous element and
//     writes the new element with no trailing comma — preserving the
//     no-trailing-comma style.
func appendAfterLast(src []byte, tup *hclsyntax.TupleConsExpr, workspace, indent, eol string) ([]byte, error) {
	last := tup.Exprs[len(tup.Exprs)-1]
	endByte := last.Range().End.Byte
	if endByte > len(src) {
		return nil, fmt.Errorf("internal: last element end byte %d out of range", endByte)
	}
	hasTrailingComma := false
	for i := endByte; i < tup.Range().End.Byte && i < len(src); i++ {
		c := src[i]
		if c == ',' {
			hasTrailingComma = true
			break
		}
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			continue
		}
		// Anything else (including the closing ']') means we passed the
		// post-element gap with no comma found.
		break
	}

	var insertion []byte
	var insertAt int
	if hasTrailingComma {
		// Insert a new well-formed line after the line containing the
		// trailing comma, so the new element also ends with a comma.
		commaLineEnd := bytes.IndexByte(src[endByte:], '\n')
		if commaLineEnd < 0 {
			return nil, fmt.Errorf("internal: no newline after trailing comma")
		}
		insertAt = endByte + commaLineEnd + 1
		insertion = []byte(indent + `"` + workspace + `",` + eol)
	} else {
		// Add the missing comma to the previous element and write the
		// new element with no trailing comma to preserve the style.
		insertAt = endByte
		insertion = []byte("," + eol + indent + `"` + workspace + `"`)
	}
	return spliceAt(src, insertAt, insertion), nil
}

// spliceAt inserts ins into src at byte offset off. Returns a fresh
// slice; src is not mutated.
func spliceAt(src []byte, off int, ins []byte) []byte {
	out := make([]byte, 0, len(src)+len(ins))
	out = append(out, src[:off]...)
	out = append(out, ins...)
	out = append(out, src[off:]...)
	return out
}
