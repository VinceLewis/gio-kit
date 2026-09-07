package form

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

var (
	ErrInvalid    = errors.New("form: validation failed")
	ErrSubmitting = errors.New("form: submission already in flight")
)

type field struct {
	schema    FieldSchema
	value     string
	loaded    string
	reference Reference
	touched   bool
	err       string
}

type Form struct {
	mu         sync.RWMutex
	order      []string
	fields     map[string]*field
	rules      []UIRule
	submit     Submitter
	onSuccess  func(map[string]string)
	onCancel   func()
	notify     func()
	submitting bool
	submitErr  error
	closed     bool
}

func New(schema []FieldSchema, rules []UIRule, notify func()) (*Form, error) {
	f := &Form{fields: make(map[string]*field, len(schema)), rules: append([]UIRule(nil), rules...), notify: notify}
	for _, definition := range schema {
		if definition.ID == "" || f.fields[definition.ID] != nil {
			return nil, fmt.Errorf("form: empty or duplicate field id %q", definition.ID)
		}
		if definition.Label == "" {
			definition.Label = definition.ID
		}
		if definition.Type == FieldChoice && len(definition.Choices) == 0 {
			return nil, fmt.Errorf("form: choice field %q has no choices", definition.ID)
		}
		if definition.Type == FieldReference && definition.RefTable == "" {
			return nil, fmt.Errorf("form: reference field %q has no table", definition.ID)
		}
		f.order = append(f.order, definition.ID)
		f.fields[definition.ID] = &field{schema: definition, value: definition.DefaultValue, loaded: definition.DefaultValue}
	}
	for _, rule := range rules {
		for _, condition := range rule.Conditions {
			if f.fields[condition.FieldID] == nil {
				return nil, fmt.Errorf("form: rule refers to unknown field %q", condition.FieldID)
			}
		}
		for _, effect := range rule.Effects {
			if f.fields[effect.FieldID] == nil {
				return nil, fmt.Errorf("form: rule affects unknown field %q", effect.FieldID)
			}
		}
	}
	return f, nil
}

func (f *Form) Load(values map[string]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id := range values {
		if f.fields[id] == nil {
			return fmt.Errorf("form: unknown field %q", id)
		}
	}
	for _, item := range f.fields {
		value := item.schema.DefaultValue
		if loaded, ok := values[item.schema.ID]; ok {
			value = loaded
		}
		item.value, item.loaded, item.err, item.touched = value, value, "", false
		item.reference = Reference{}
	}
	f.submitErr = nil
	return nil
}

func (f *Form) SetValue(id, value string) error {
	f.mu.Lock()
	item := f.fields[id]
	if item == nil {
		f.mu.Unlock()
		return fmt.Errorf("form: unknown field %q", id)
	}
	_, _, readOnly := f.effectiveLocked(id)
	if readOnly {
		f.mu.Unlock()
		return fmt.Errorf("form: field %q is read-only", id)
	}
	item.value, item.err = value, ""
	item.reference = Reference{}
	notify := f.notify
	f.mu.Unlock()
	if notify != nil {
		notify()
	}
	return nil
}

func (f *Form) SetReference(id, recordID, display string) error {
	if err := f.SetValue(id, recordID); err != nil {
		return err
	}
	f.mu.Lock()
	f.fields[id].reference = Reference{ID: recordID, Display: display}
	f.mu.Unlock()
	return nil
}

func (f *Form) Blur(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	item := f.fields[id]
	if item == nil {
		return fmt.Errorf("form: unknown field %q", id)
	}
	item.touched = true
	item.err = f.validateFieldLocked(id)
	return nil
}

func (f *Form) SetFieldError(id, message string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	item := f.fields[id]
	if item == nil {
		return fmt.Errorf("form: unknown field %q", id)
	}
	item.touched, item.err = true, message
	return nil
}

func (f *Form) Values() map[string]string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.valuesLocked()
}

func (f *Form) IsDirty() bool { return f.Snapshot().Dirty }
func (f *Form) IsValid() bool { return f.Snapshot().Valid }

func (f *Form) SetSubmitter(submit Submitter) {
	f.mu.Lock()
	f.submit = submit
	f.mu.Unlock()
}

func (f *Form) OnSuccess(fn func(map[string]string)) {
	f.mu.Lock()
	f.onSuccess = fn
	f.mu.Unlock()
}

func (f *Form) OnCancel(fn func()) {
	f.mu.Lock()
	f.onCancel = fn
	f.mu.Unlock()
}

func (f *Form) Cancel() {
	f.mu.RLock()
	fn := f.onCancel
	f.mu.RUnlock()
	if fn != nil {
		fn()
	}
}

func (f *Form) Submit(ctx context.Context) error {
	f.mu.Lock()
	if f.submitting {
		f.mu.Unlock()
		return ErrSubmitting
	}
	valid := true
	for id, item := range f.fields {
		item.touched = true
		item.err = f.validateFieldLocked(id)
		valid = valid && item.err == ""
	}
	if !valid {
		f.mu.Unlock()
		return ErrInvalid
	}
	if f.submit == nil {
		f.mu.Unlock()
		return errors.New("form: submitter is not configured")
	}
	f.submitting, f.submitErr = true, nil
	values, submit := f.valuesLocked(), f.submit
	notify := f.notify
	f.mu.Unlock()
	if notify != nil {
		notify()
	}
	go f.runSubmit(ctx, submit, values)
	return nil
}

func (f *Form) runSubmit(ctx context.Context, submit Submitter, values map[string]string) {
	err := submit(ctx, values)
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return
	}
	f.submitting, f.submitErr = false, err
	if fieldErrors, ok := err.(FieldErrors); ok {
		for id, message := range fieldErrors {
			if item := f.fields[id]; item != nil {
				item.touched, item.err = true, message
			}
		}
	}
	var success func(map[string]string)
	if err == nil {
		for _, item := range f.fields {
			item.loaded = values[item.schema.ID]
		}
		success = f.onSuccess
	}
	notify := f.notify
	f.mu.Unlock()
	if notify != nil {
		notify()
	}
	if success != nil {
		success(values)
	}
}

func (f *Form) Snapshot() Snapshot {
	f.mu.RLock()
	defer f.mu.RUnlock()
	result := Snapshot{Values: f.valuesLocked(), Valid: true, Submitting: f.submitting, SubmitError: f.submitErr}
	for _, id := range f.order {
		item := f.fields[id]
		visible, mandatory, readOnly := f.effectiveLocked(id)
		err := item.err
		if validation := f.validateFieldLocked(id); validation != "" {
			result.Valid = false
			if item.touched {
				err = validation
			}
		}
		dirty := item.value != item.loaded
		result.Dirty = result.Dirty || dirty
		result.Fields = append(result.Fields, FieldState{
			Schema: item.schema, Value: item.value, Reference: item.reference,
			Visible: visible, Mandatory: mandatory, ReadOnly: readOnly,
			Dirty: dirty, Touched: item.touched, Error: err,
		})
	}
	return result
}

func (f *Form) Close() {
	f.mu.Lock()
	f.closed = true
	f.mu.Unlock()
}

func (f *Form) effectiveLocked(id string) (visible, mandatory, readOnly bool) {
	item := f.fields[id]
	visible, mandatory, readOnly = true, item.schema.Mandatory, item.schema.ReadOnly
	for _, rule := range f.rules {
		matches := true
		for _, condition := range rule.Conditions {
			equal := f.fields[condition.FieldID].value == condition.Equals
			if condition.Not {
				equal = !equal
			}
			matches = matches && equal
		}
		if !matches {
			continue
		}
		for _, effect := range rule.Effects {
			if effect.FieldID != id {
				continue
			}
			if effect.SetVisible {
				visible = effect.Visible
			}
			if effect.SetMandatory {
				mandatory = effect.Mandatory
			}
			if effect.SetReadOnly {
				readOnly = effect.ReadOnly
			}
		}
	}
	return
}

func (f *Form) validateFieldLocked(id string) string {
	item := f.fields[id]
	visible, mandatory, _ := f.effectiveLocked(id)
	if !visible {
		return ""
	}
	if mandatory && strings.TrimSpace(item.value) == "" {
		return "Required"
	}
	if item.value == "" {
		return ""
	}
	for _, validator := range item.schema.Validators {
		if ok, message := validator(item.value); !ok {
			return message
		}
	}
	return ""
}

func (f *Form) valuesLocked() map[string]string {
	values := make(map[string]string, len(f.fields))
	for id, item := range f.fields {
		values[id] = item.value
	}
	return values
}
