// Package form provides schema-driven CRUD form state, validation, UI rules,
// asynchronous submission, and an optional Gio renderer.
package form

import "context"

type FieldType uint8

const (
	FieldText FieldType = iota
	FieldTextArea
	FieldNumber
	FieldBoolean
	FieldDate
	FieldDateTime
	FieldChoice
	FieldReference
	FieldTime
	FieldAttachment
)

type Choice struct {
	Value string
	Label string
}

type Reference struct {
	ID      string
	Display string
}

type Validator func(value string) (ok bool, message string)

type FieldSchema struct {
	ID                string
	Label             string
	Type              FieldType
	Mandatory         bool
	ReadOnly          bool
	DefaultValue      string
	Choices           []Choice
	RefTable          string
	Validators        []Validator
	UnavailableReason string
}

type Condition struct {
	FieldID string
	Equals  string
	Not     bool
}

// Effect changes only properties whose Set flag is true.
type Effect struct {
	FieldID      string
	SetVisible   bool
	Visible      bool
	SetMandatory bool
	Mandatory    bool
	SetReadOnly  bool
	ReadOnly     bool
}

// UIRule applies Effects when all Conditions match current values.
type UIRule struct {
	Conditions []Condition
	Effects    []Effect
}

type FieldState struct {
	Schema    FieldSchema
	Value     string
	Loaded    string
	Reference Reference
	Visible   bool
	Mandatory bool
	ReadOnly  bool
	Dirty     bool
	Touched   bool
	Error     string
}

type Snapshot struct {
	Fields      []FieldState
	Values      map[string]string
	Valid       bool
	Dirty       bool
	Submitting  bool
	SubmitError error
}

type Submitter func(context.Context, map[string]string) error

// FieldErrors represents backend validation errors keyed by field ID.
type FieldErrors map[string]string

func (e FieldErrors) Error() string { return "form: submission contains field errors" }
