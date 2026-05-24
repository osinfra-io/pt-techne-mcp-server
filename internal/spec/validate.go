// JSON Schema validation for team specs.
package spec

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

//go:embed schema_embed.json
var schemaBytes []byte

// ValidationError is a single human-readable validation failure tied to a
// JSON Pointer path within the input spec.
type ValidationError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// Validator validates JSON-decoded team specs against the embedded schema.
// Construct via NewValidator; safe for concurrent use after construction.
type Validator struct {
	schema *jsonschema.Schema
	// printer is a non-nil i18n printer used for jsonschema's LocalizedString.
	// The library panics on a nil printer for some error kinds, so we always
	// pass an English printer.
	printer *message.Printer
}

// NewValidator compiles the embedded JSON Schema. Returns an error if the
// schema itself fails to compile (a build-time bug, not a runtime input issue).
func NewValidator() (*Validator, error) {
	c := jsonschema.NewCompiler()
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaBytes))
	if err != nil {
		return nil, fmt.Errorf("decode embedded schema: %w", err)
	}
	if err := c.AddResource("team.schema.json", doc); err != nil {
		return nil, fmt.Errorf("add embedded schema: %w", err)
	}
	s, err := c.Compile("team.schema.json")
	if err != nil {
		return nil, fmt.Errorf("compile embedded schema: %w", err)
	}
	return &Validator{schema: s, printer: message.NewPrinter(language.English)}, nil
}

// Validate validates a JSON-decoded spec (interface{} from json.Unmarshal).
// Returns nil + empty errors when valid; never panics. A nil receiver or
// uninitialised validator returns a single configuration error rather than
// panicking — callers that fail to construct via NewValidator surface a clear
// message.
func (v *Validator) Validate(spec any) []ValidationError {
	if v == nil || v.schema == nil {
		return []ValidationError{{Path: "", Message: "validator not initialised; call NewValidator first"}}
	}
	if err := v.schema.Validate(spec); err != nil {
		var verr *jsonschema.ValidationError
		if errorsAs(err, &verr) {
			return flatten(verr, v.printer)
		}
		return []ValidationError{{Path: "", Message: err.Error()}}
	}
	return nil
}

// ValidateJSON parses raw JSON then validates. Returns a parse error before
// validation if JSON is malformed.
func (v *Validator) ValidateJSON(data []byte) ([]ValidationError, error) {
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("parse spec JSON: %w", err)
	}
	return v.Validate(parsed), nil
}

// errorsAs is a thin wrapper to avoid importing errors at top — keeps imports tight.
// Supports both single Unwrap() error and multi Unwrap() []error chains.
func errorsAs(err error, target **jsonschema.ValidationError) bool {
	for e := err; e != nil; {
		if v, ok := e.(*jsonschema.ValidationError); ok {
			*target = v
			return true
		}
		// Try single-error unwrap first.
		type unwrapper interface{ Unwrap() error }
		if u, ok := e.(unwrapper); ok {
			e = u.Unwrap()
			continue
		}
		// Try multi-error unwrap (Go 1.20+).
		type multiUnwrapper interface{ Unwrap() []error }
		if mu, ok := e.(multiUnwrapper); ok {
			for _, inner := range mu.Unwrap() {
				if errorsAs(inner, target) {
					return true
				}
			}
		}
		return false
	}
	return false
}

// flatten walks the validation error tree and returns one entry per leaf
// failure with a JSON Pointer path. Output is sorted by path for determinism.
func flatten(root *jsonschema.ValidationError, printer *message.Printer) []ValidationError {
	var out []ValidationError
	var walk func(*jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			out = append(out, ValidationError{
				Path:    pointer(e.InstanceLocation),
				Message: e.ErrorKind.LocalizedString(printer),
			})
			return
		}
		for _, c := range e.Causes {
			walk(c)
		}
	}
	walk(root)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Message < out[j].Message
	})
	return out
}

// pointer joins a slice instance-location into a JSON Pointer.
func pointer(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	var b bytes.Buffer
	for _, p := range parts {
		b.WriteByte('/')
		// JSON Pointer escapes
		for _, r := range p {
			switch r {
			case '~':
				b.WriteString("~0")
			case '/':
				b.WriteString("~1")
			default:
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}
