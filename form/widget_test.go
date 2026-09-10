package form

import (
	"context"
	"testing"
	"time"
)

func TestFastSuccessfulSubmitCompletesPendingClose(t *testing.T) {
	form := testForm(t)
	if err := form.Load(map[string]string{"description": "before", "priority": "2", "category": "software"}); err != nil {
		t.Fatal(err)
	}
	if err := form.SetValue("description", "after"); err != nil {
		t.Fatal(err)
	}
	form.SetSubmitter(func(context.Context, map[string]string) error { return nil })
	widget := NewWidget(form)
	widget.pendingClose = true
	closed := false
	widget.OnSaveAndClose = func() { closed = true }
	if err := form.Submit(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for form.Snapshot().Submitting && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	widget.completePendingClose(form.Snapshot())
	if !closed || widget.pendingClose {
		t.Fatalf("pending close was not completed: closed=%t pending=%t", closed, widget.pendingClose)
	}
}

func TestUpdateSubmitGatingIncludesStagedChanges(t *testing.T) {
	widget := NewWidget(testForm(t))
	widget.RequireDirtySubmit = true
	if !widget.submitDisabled(Snapshot{Valid: true}) {
		t.Fatal("clean update save remained enabled")
	}
	widget.AdditionalDirty = true
	if widget.submitDisabled(Snapshot{Valid: true}) {
		t.Fatal("staged child change did not enable save")
	}
	if !widget.submitDisabled(Snapshot{Valid: false, Dirty: true}) || !widget.submitDisabled(Snapshot{Valid: true, Dirty: true, Submitting: true}) {
		t.Fatal("invalid or submitting update save remained enabled")
	}
	create := NewWidget(testForm(t))
	if create.submitDisabled(Snapshot{Valid: true}) {
		t.Fatal("pristine valid create was disabled")
	}
}
