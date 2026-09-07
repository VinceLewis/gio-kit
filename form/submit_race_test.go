package form

import (
	"context"
	"testing"
	"time"
)

func TestF5EditDuringSubmitRemainsDirty(t *testing.T) {
	f := testForm(t)
	_ = f.Load(map[string]string{"description": "first", "priority": "2", "category": "software"})
	release := make(chan struct{})
	f.SetSubmitter(func(context.Context, map[string]string) error {
		<-release
		return nil
	})
	if err := f.Submit(context.Background()); err != nil {
		t.Fatal(err)
	}
	_ = f.SetValue("description", "second")
	close(release)
	deadline := time.Now().Add(time.Second)
	for f.Snapshot().Submitting && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !f.IsDirty() {
		t.Fatal("edit made during submit was incorrectly marked clean")
	}
}
