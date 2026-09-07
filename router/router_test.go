package router

import (
	"errors"
	"sync"
	"testing"
)

func testRouter(t *testing.T) *Router {
	t.Helper()
	table, err := NewTable(
		Definition{Name: "incident.list", Pattern: "/incidents"},
		Definition{Name: "incident.form", Pattern: "/incident/:id"},
		Definition{Name: "incident.new", Pattern: "/incident/new"},
	)
	if err != nil {
		t.Fatal(err)
	}
	r, err := New(table, Route{Name: "incident.list", Params: Params{"filter": String("active")}})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestN1TypedParametersAndDefensiveCopies(t *testing.T) {
	r := testRouter(t)
	params := Params{"id": String("abc"), "page": Int(7), "preview": Bool(true)}
	if err := r.Push(Route{Name: "incident.form", Params: params}); err != nil {
		t.Fatal(err)
	}
	params["id"] = String("mutated")
	got := r.Current().Route.Params
	if got["id"].String() != "abc" {
		t.Fatalf("history was mutated: %v", got)
	}
	page, err := got["page"].Int64()
	if err != nil || page != 7 {
		t.Fatalf("typed page = %d, %v", page, err)
	}
}

func TestN2HistoryRetainsTransientScreenState(t *testing.T) {
	r := testRouter(t)
	state := &struct{ Scroll int }{Scroll: 42}
	if err := r.SetCurrentState(state); err != nil {
		t.Fatal(err)
	}
	if err := r.Push(Route{Name: "incident.form", Params: Params{"id": String("1")}}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Pop(); err != nil {
		t.Fatal(err)
	}
	if r.Current().State != state {
		t.Fatal("previous screen state identity was not preserved")
	}
}

func TestN4ResolveDeepLinkWithoutNavigation(t *testing.T) {
	r := testRouter(t)
	got, err := r.ResolveURL("gio-kit://app/incident/sys_id_123?openedFrom=notification")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "incident.form" || got.Params["id"].String() != "sys_id_123" || got.Params["openedFrom"].String() != "notification" {
		t.Fatalf("unexpected route: %#v", got)
	}
	if len(r.Stack()) != 1 {
		t.Fatal("resolution should not navigate")
	}
}

func TestN5SerializeRestoreAndRejectInvalidState(t *testing.T) {
	r := testRouter(t)
	_ = r.Push(Route{Name: "incident.form", Params: Params{"id": String("77")}})
	_ = r.PushModal(Route{Name: "incident.new"})
	data, err := r.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	restored := testRouter(t)
	if err := restored.Restore(data); err != nil {
		t.Fatal(err)
	}
	if restored.MainCurrent().Route.Params["id"].String() != "77" || len(restored.Modals()) != 1 {
		t.Fatalf("restore mismatch: stack=%v modals=%v", restored.Stack(), restored.Modals())
	}
	before, _ := restored.Serialize()
	if err := restored.Restore([]byte(`{"version":99,"stack":[]}`)); !errors.Is(err, ErrBadState) {
		t.Fatalf("invalid restore error = %v", err)
	}
	after, _ := restored.Serialize()
	if string(before) != string(after) {
		t.Fatal("failed restore mutated router")
	}
}

func TestN6ModalDoesNotDisturbMainHistory(t *testing.T) {
	r := testRouter(t)
	state := new(int)
	_ = r.SetCurrentState(state)
	_ = r.PushModal(Route{Name: "incident.new"})
	if r.Current().Route.Name != "incident.new" || r.MainCurrent().Route.Name != "incident.list" {
		t.Fatal("modal and main stacks are not independent")
	}
	if _, err := r.PopModal(); err != nil {
		t.Fatal(err)
	}
	if r.Current().State != state {
		t.Fatal("modal dismissal rebuilt underlying state")
	}
}

func TestN7GuardBlocksAndForcePopConfirms(t *testing.T) {
	r := testRouter(t)
	_ = r.Push(Route{Name: "incident.form", Params: Params{"id": String("1")}})
	r.SetGuard(func(transition Transition) bool { return transition.Operation != PopOperation })
	if _, err := r.Pop(); !errors.Is(err, ErrGuarded) {
		t.Fatalf("guarded pop error = %v", err)
	}
	if r.Current().Route.Name != "incident.form" {
		t.Fatal("blocked navigation mutated history")
	}
	if _, err := r.ForcePop(); err != nil || r.Current().Route.Name != "incident.list" {
		t.Fatalf("confirmed pop failed: %v", err)
	}
}

func TestN8ConcurrentExternalPush(t *testing.T) {
	r := testRouter(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			_ = r.Push(Route{Name: "incident.form", Params: Params{"sequence": Int(id)}})
		}(int64(i))
	}
	wg.Wait()
	if got := len(r.Stack()); got != 21 {
		t.Fatalf("stack length = %d, want 21", got)
	}
}
