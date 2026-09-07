package router

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

var (
	ErrGuarded   = errors.New("router: navigation blocked by guard")
	ErrRoot      = errors.New("router: cannot pop root route")
	ErrNoModal   = errors.New("router: modal stack is empty")
	ErrEmpty     = errors.New("router: route stack is empty")
	ErrBadState  = errors.New("router: invalid restored state")
	ErrUnmatched = errors.New("router: route is not registered")
)

// Route is a named destination with typed, serializable parameters.
type Route struct {
	Name   string `json:"name"`
	Params Params `json:"params,omitempty"`
}

func (r Route) clone() Route { return Route{Name: r.Name, Params: r.Params.clone()} }

// Entry couples a serializable route with transient screen state. State is
// retained while the entry is in history but deliberately omitted on restore.
type Entry struct {
	Route Route
	State any
}

// Operation identifies the mutation offered to a guard.
type Operation string

const (
	PushOperation      Operation = "push"
	PopOperation       Operation = "pop"
	ReplaceOperation   Operation = "replace"
	PushModalOperation Operation = "push-modal"
	PopModalOperation  Operation = "pop-modal"
)

// Transition describes a proposed navigation mutation.
type Transition struct {
	Operation Operation
	From      Route
	To        Route
}

type Guard func(Transition) bool

const stateVersion = 1

type savedState struct {
	Version int     `json:"version"`
	Stack   []Route `json:"stack"`
	Modals  []Route `json:"modals,omitempty"`
}

// Router is safe to call from UI and background goroutines.
type Router struct {
	mu      sync.RWMutex
	table   *Table
	stack   []Entry
	modals  []Entry
	guard   Guard
	version uint64
}

func New(table *Table, initial Route) (*Router, error) {
	if table == nil {
		return nil, errors.New("router: route table is nil")
	}
	r := &Router{table: table}
	if err := r.validate(initial); err != nil {
		return nil, err
	}
	r.stack = []Entry{{Route: initial.clone()}}
	return r, nil
}

func (r *Router) ResolveURL(rawURL string) (Route, error) { return r.table.ResolveURL(rawURL) }

func (r *Router) SetGuard(guard Guard) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.guard = guard
}

func (r *Router) Version() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.version
}

func (r *Router) Current() Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.modals) > 0 {
		return cloneEntry(r.modals[len(r.modals)-1])
	}
	if len(r.stack) == 0 {
		return Entry{}
	}
	return cloneEntry(r.stack[len(r.stack)-1])
}

func (r *Router) MainCurrent() Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.stack) == 0 {
		return Entry{}
	}
	return cloneEntry(r.stack[len(r.stack)-1])
}

func (r *Router) Stack() []Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return cloneEntries(r.stack)
}

func (r *Router) Modals() []Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return cloneEntries(r.modals)
}

func (r *Router) SetCurrentState(state any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.modals) > 0 {
		r.modals[len(r.modals)-1].State = state
		return nil
	}
	if len(r.stack) == 0 {
		return ErrEmpty
	}
	r.stack[len(r.stack)-1].State = state
	return nil
}

func (r *Router) Push(route Route) error { return r.PushWithState(route, nil) }

func (r *Router) PushWithState(route Route, state any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.validate(route); err != nil {
		return err
	}
	from := r.currentLocked().Route
	if !r.allowedLocked(Transition{Operation: PushOperation, From: from, To: route}) {
		return ErrGuarded
	}
	r.stack = append(r.stack, Entry{Route: route.clone(), State: state})
	r.version++
	return nil
}

func (r *Router) Pop() (Entry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.modals) > 0 {
		return r.popModalLocked(false)
	}
	if len(r.stack) <= 1 {
		return Entry{}, ErrRoot
	}
	from := r.stack[len(r.stack)-1].Route
	to := r.stack[len(r.stack)-2].Route
	if !r.allowedLocked(Transition{Operation: PopOperation, From: from, To: to}) {
		return Entry{}, ErrGuarded
	}
	removed := r.stack[len(r.stack)-1]
	r.stack = r.stack[:len(r.stack)-1]
	r.version++
	return cloneEntry(removed), nil
}

func (r *Router) Replace(route Route) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.validate(route); err != nil {
		return err
	}
	if len(r.stack) == 0 {
		return ErrEmpty
	}
	from := r.stack[len(r.stack)-1].Route
	if !r.allowedLocked(Transition{Operation: ReplaceOperation, From: from, To: route}) {
		return ErrGuarded
	}
	r.stack[len(r.stack)-1] = Entry{Route: route.clone()}
	r.modals = nil
	r.version++
	return nil
}

func (r *Router) PushModal(route Route) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.validate(route); err != nil {
		return err
	}
	from := r.currentLocked().Route
	if !r.allowedLocked(Transition{Operation: PushModalOperation, From: from, To: route}) {
		return ErrGuarded
	}
	r.modals = append(r.modals, Entry{Route: route.clone()})
	r.version++
	return nil
}

func (r *Router) PopModal() (Entry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.popModalLocked(false)
}

// ForcePop repeats a user-confirmed back action without consulting the guard.
func (r *Router) ForcePop() (Entry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.modals) > 0 {
		return r.popModalLocked(true)
	}
	if len(r.stack) <= 1 {
		return Entry{}, ErrRoot
	}
	removed := r.stack[len(r.stack)-1]
	r.stack = r.stack[:len(r.stack)-1]
	r.version++
	return cloneEntry(removed), nil
}

func (r *Router) popModalLocked(force bool) (Entry, error) {
	if len(r.modals) == 0 {
		return Entry{}, ErrNoModal
	}
	from := r.modals[len(r.modals)-1].Route
	var to Route
	if len(r.modals) > 1 {
		to = r.modals[len(r.modals)-2].Route
	} else if len(r.stack) > 0 {
		to = r.stack[len(r.stack)-1].Route
	}
	if !force && !r.allowedLocked(Transition{Operation: PopModalOperation, From: from, To: to}) {
		return Entry{}, ErrGuarded
	}
	removed := r.modals[len(r.modals)-1]
	r.modals = r.modals[:len(r.modals)-1]
	r.version++
	return cloneEntry(removed), nil
}

func (r *Router) Serialize() ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	state := savedState{Version: stateVersion, Stack: routesOf(r.stack), Modals: routesOf(r.modals)}
	return json.Marshal(state)
}

func (r *Router) Restore(data []byte) error {
	var state savedState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("%w: %v", ErrBadState, err)
	}
	if state.Version != stateVersion || len(state.Stack) == 0 {
		return ErrBadState
	}
	for _, route := range append(append([]Route(nil), state.Stack...), state.Modals...) {
		if err := r.validate(route); err != nil {
			return fmt.Errorf("%w: %v", ErrBadState, err)
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stack = entriesOf(state.Stack)
	r.modals = entriesOf(state.Modals)
	r.version++
	return nil
}

func (r *Router) validate(route Route) error {
	if route.Name == "" || !r.table.Has(route.Name) {
		return fmt.Errorf("%w: %q", ErrUnmatched, route.Name)
	}
	for key, value := range route.Params {
		if key == "" || !value.valid() {
			return fmt.Errorf("router: invalid parameter %q", key)
		}
	}
	return nil
}

func (r *Router) currentLocked() Entry {
	if len(r.modals) > 0 {
		return r.modals[len(r.modals)-1]
	}
	if len(r.stack) > 0 {
		return r.stack[len(r.stack)-1]
	}
	return Entry{}
}

func (r *Router) allowedLocked(transition Transition) bool {
	return r.guard == nil || r.guard(transition)
}

func cloneEntry(entry Entry) Entry {
	entry.Route = entry.Route.clone()
	return entry
}

func cloneEntries(entries []Entry) []Entry {
	cloned := make([]Entry, len(entries))
	for i, entry := range entries {
		cloned[i] = cloneEntry(entry)
	}
	return cloned
}

func routesOf(entries []Entry) []Route {
	routes := make([]Route, len(entries))
	for i, entry := range entries {
		routes[i] = entry.Route.clone()
	}
	return routes
}

func entriesOf(routes []Route) []Entry {
	entries := make([]Entry, len(routes))
	for i, route := range routes {
		entries[i] = Entry{Route: route.clone()}
	}
	return entries
}
