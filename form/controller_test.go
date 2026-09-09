package form

import (
	"context"
	"errors"
	"testing"
	"time"
)

func testForm(t *testing.T) *Form {
	t.Helper()
	f, err := New([]FieldSchema{
		{ID: "description", Type: FieldText, Mandatory: true, Validators: []Validator{MinLength(3), MaxLength(10)}},
		{ID: "category", Type: FieldChoice, Choices: []Choice{{Value: "software"}, {Value: "hardware"}}},
		{ID: "configuration_item", Type: FieldReference, RefTable: "incident"},
		{ID: "priority", Type: FieldNumber, Validators: []Validator{NumberRange(1, 4)}},
		{ID: "readonly", Type: FieldText, ReadOnly: true},
	}, []UIRule{{
		Conditions: []Condition{{FieldID: "category", Equals: "hardware"}},
		Effects:    []Effect{{FieldID: "configuration_item", SetVisible: true, Visible: true, SetMandatory: true, Mandatory: true}},
	}, {
		Conditions: []Condition{{FieldID: "category", Equals: "hardware", Not: true}},
		Effects:    []Effect{{FieldID: "configuration_item", SetVisible: true, Visible: false}},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestF1F2SchemaAndTypes(t *testing.T) {
	f := testForm(t)
	if got := len(f.Snapshot().Fields); got != 5 {
		t.Fatalf("fields=%d", got)
	}
}

func TestF3F4Validation(t *testing.T) {
	f := testForm(t)
	if f.IsValid() {
		t.Fatal("empty required form is valid")
	}
	_ = f.SetValue("description", "ok")
	_ = f.Blur("description")
	if f.Snapshot().Fields[0].Error == "" {
		t.Fatal("blur did not expose validation")
	}
	_ = f.SetValue("description", "valid")
	_ = f.SetValue("priority", "5")
	if f.IsValid() {
		t.Fatal("out-of-range number is valid")
	}
}

func TestF5DirtyLoad(t *testing.T) {
	f := testForm(t)
	_ = f.Load(map[string]string{"description": "loaded", "priority": "2"})
	if f.IsDirty() {
		t.Fatal("loaded form is dirty")
	}
	_ = f.SetValue("description", "changed")
	if !f.IsDirty() {
		t.Fatal("changed form is clean")
	}
	_ = f.SetValue("description", "loaded")
	if f.IsDirty() {
		t.Fatal("restored value is dirty")
	}
}

func TestF6RulesReevaluate(t *testing.T) {
	f := testForm(t)
	_ = f.SetValue("description", "valid")
	_ = f.SetValue("priority", "2")
	_ = f.SetValue("category", "software")
	if state := f.Snapshot().Fields[2]; state.Visible || state.Mandatory {
		t.Fatalf("software reference state=%+v", state)
	}
	_ = f.SetValue("category", "hardware")
	if state := f.Snapshot().Fields[2]; !state.Visible || !state.Mandatory {
		t.Fatalf("hardware reference state=%+v", state)
	}
}

func TestF7ReferenceStoresIDAndDisplay(t *testing.T) {
	f := testForm(t)
	if err := f.SetReference("configuration_item", "sys_1", "INC0001"); err != nil {
		t.Fatal(err)
	}
	state := f.Snapshot().Fields[2]
	if state.Value != "sys_1" || state.Reference.Display != "INC0001" {
		t.Fatalf("reference=%+v", state)
	}
}

func TestF8SubmitLifecycleAndF10ServerErrors(t *testing.T) {
	f := testForm(t)
	_ = f.Load(map[string]string{"description": "valid", "priority": "2", "category": "software"})
	done := make(chan struct{})
	f.SetSubmitter(func(context.Context, map[string]string) error {
		defer close(done)
		return FieldErrors{"description": "Already exists"}
	})
	if err := f.Submit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := f.Submit(context.Background()); !errors.Is(err, ErrSubmitting) {
		t.Fatalf("second submit=%v", err)
	}
	<-done
	deadline := time.Now().Add(time.Second)
	for f.Snapshot().Submitting && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := f.Snapshot().Fields[0].Error; got != "Already exists" {
		t.Fatalf("backend field error=%q", got)
	}
}

func TestFieldErrorsExplainFieldsDeterministically(t *testing.T) {
	err := FieldErrors{"zeta": "", "alpha": "Already exists"}
	if got, want := err.Error(), "alpha: Already exists; zeta: Invalid value"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestF9SecondRecordUsesOnlySchema(t *testing.T) {
	f, err := New([]FieldSchema{{ID: "risk", Type: FieldNumber}}, nil, nil)
	if err != nil || len(f.Snapshot().Fields) != 1 {
		t.Fatalf("new record schema: %v", err)
	}
}

func TestTimeAndAttachmentFieldsRetainValues(t *testing.T) {
	f, err := New([]FieldSchema{{ID: "at", Type: FieldTime}, {ID: "file", Type: FieldAttachment}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Load(map[string]string{"at": "14:30", "file": "document.pdf"}); err != nil {
		t.Fatal(err)
	}
	values := f.Snapshot().Values
	if values["at"] != "14:30" || values["file"] != "document.pdf" {
		t.Fatalf("values = %#v", values)
	}
}
