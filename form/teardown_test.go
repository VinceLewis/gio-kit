package form

import (
	"context"
	"testing"
)

func TestCloseCancelsAndWaitJoinsSubmission(t *testing.T) {
	f, err := New([]FieldSchema{{ID: "value"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	started, finished := make(chan struct{}), make(chan struct{})
	f.SetSubmitter(func(ctx context.Context, _ map[string]string) error {
		close(started)
		<-ctx.Done()
		close(finished)
		return ctx.Err()
	})
	if err = f.Submit(context.Background()); err != nil {
		t.Fatal(err)
	}
	<-started
	if !f.Pending() {
		t.Fatal("active submission reported idle")
	}
	f.Close()
	f.Wait()
	select {
	case <-finished:
	default:
		t.Fatal("Wait returned before submitter")
	}
	if f.Pending() {
		t.Fatal("closed submission remained pending")
	}
	if err = f.Submit(context.Background()); err == nil {
		t.Fatal("closed form accepted work")
	}
}
