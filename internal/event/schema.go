package event

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed schema/event.json
var schemaJSON []byte

// schemaURL is the $id declared inside schema/event.json.
const schemaURL = "https://techlog.example.invalid/schema/0.1-draft/event.json"

// SchemaJSON returns a copy of the embedded draft 0.1 event schema.
func SchemaJSON() []byte { return bytes.Clone(schemaJSON) }

// Problem is one validation failure. Path is a JSON Pointer into the document
// ("/" for the document itself).
type Problem struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// ValidationError lists every problem found in a document.
type ValidationError struct {
	Problems []Problem
}

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		parts[i] = p.Path + ": " + p.Message
	}
	return "invalid: " + strings.Join(parts, "; ")
}

// Validator checks documents against the embedded schema. It is safe for
// concurrent use.
type Validator struct {
	event *jsonschema.Schema
	link  *jsonschema.Schema
}

// NewValidator compiles the embedded schema once.
func NewValidator() (*Validator, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))
	if err != nil {
		return nil, fmt.Errorf("parse embedded schema: %w", err)
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat() // enforce "date-time"; formats are annotations by default
	if err := c.AddResource(schemaURL, doc); err != nil {
		return nil, fmt.Errorf("add schema: %w", err)
	}
	ev, err := c.Compile(schemaURL)
	if err != nil {
		return nil, fmt.Errorf("compile event schema: %w", err)
	}
	link, err := c.Compile(schemaURL + "#/$defs/link")
	if err != nil {
		return nil, fmt.Errorf("compile link schema: %w", err)
	}
	return &Validator{event: ev, link: link}, nil
}

// Validate checks an event document. The error is a *ValidationError.
func (v *Validator) Validate(doc map[string]any) error {
	return validate(v.event, doc)
}

// ValidateLink checks one link ({rel, target, basis}).
func (v *Validator) ValidateLink(doc map[string]any) error {
	return validate(v.link, doc)
}

func validate(s *jsonschema.Schema, doc map[string]any) error {
	inst, err := Normalize(doc)
	if err != nil {
		return err
	}
	err = s.Validate(inst)
	if err == nil {
		return nil
	}
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return err
	}
	var problems []Problem
	seen := map[Problem]bool{}
	leafProblems(ve.DetailedOutput(), func(p Problem) {
		if !seen[p] {
			seen[p] = true
			problems = append(problems, p)
		}
	})
	if len(problems) == 0 {
		problems = []Problem{{Path: "/", Message: err.Error()}}
	}
	return &ValidationError{Problems: problems}
}

// leafProblems reports the innermost failures of an output tree. The outer
// units of a failure reached through "$ref" only say "validation failed"; the
// leaves carry the real message, such as "value must be one of 'human', ...".
func leafProblems(u *jsonschema.OutputUnit, add func(Problem)) {
	if len(u.Errors) == 0 {
		if u.Error != nil {
			path := u.InstanceLocation
			if path == "" {
				path = "/"
			}
			add(Problem{Path: path, Message: fmt.Sprint(u.Error)})
		}
		return
	}
	for i := range u.Errors {
		leafProblems(&u.Errors[i], add)
	}
}

// Normalize returns a deep copy of doc made only of JSON types (maps, slices,
// strings, json.Number, bools, nil), so callers may pass Go ints and the
// validator never sees an unsupported type. A nil doc becomes an empty map.
func Normalize(doc map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var out map[string]any
	if err := dec.Decode(&out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}
