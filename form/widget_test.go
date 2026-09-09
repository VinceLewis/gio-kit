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
