# Gio CRUD Framework — Requirements Specification

**Scope:** Three building blocks missing from the Gio ecosystem that are required to build multi-form CRUD applications (ServiceNow-style: list view → record view → related lists, with metadata-driven forms).

1. Navigation & Routing
2. Data Grid
3. Data-Driven Form Abstraction

Each section below specifies the requirement independently so it can be built (or evaluated against a candidate library) on its own, but Section 4 shows how they compose end-to-end.

---

## 1. Navigation & Routing

### 1.1 Reference model

- **Flutter:** `Navigator 2.0` / `go_router`. Routes are named and parameterized (`/incident/:id`), pushed/popped onto a stack, resolvable from a URL for deep linking, and restorable across process death via `RouteInformationParser` + `RestorationMixin`.
- **ServiceNow:** every screen has a addressable state — list view (`incident_list.do`), form view (`incident.do?sys_id=<id>`), a "condition" (filter) carried in the URL, and breadcrumb-based back navigation. Opening a record from a related list pushes onto a visible navigation history the user can step back through.

### 1.2 Functional requirements

| # | Requirement | Notes |
|---|---|---|
| N1 | **Route table** — a route is a name plus a typed parameter set, not a bare screen enum. | `Route{Name: "incident.form", Params: {ID: "sys_id_123"}}` |
| N2 | **Stack-based history** — push/pop/replace, with the current stack inspectable (for breadcrumbs). | Pop must return to the *previous* screen with its state (scroll position, filters) intact, not a fresh instance. |
| N3 | **Parameterized navigation** — pushing a route can carry arbitrary typed data (record ID, pre-filled filter, "opened from" context). | Equivalent to Flutter's `Navigator.pushNamed(context, '/incident', arguments: id)`. |
| N4 | **Deep link resolution** — a URL-shaped string (from an OS intent, notification, or pasted link) resolves to a route + params without having navigated there interactively first. | `gio://app/incident/sys_id_123` → `Route{Name:"incident.form", Params:{ID:"sys_id_123"}}` |
| N5 | **State restoration** — after process death (mobile OS reclaiming memory) the app can reopen on the same route with the same params. | Requires the current stack to be serializable to a small persisted blob (route names + params only, not widget state). |
| N6 | **Modal / nested stacks** — a form or dialog can be pushed as an overlay above the current screen, and dismissing it returns to the underlying screen without rebuilding it. | E.g. "New Incident" opened as a modal over the list; on cancel the list's scroll/filter state is untouched. |
| N7 | **Guarded navigation** — a route push/pop can be intercepted (e.g. "unsaved changes — leave anyway?") before it completes. | Equivalent to Flutter's `WillPopScope` / `PopScope`. |
| N8 | **Programmatic + declarative triggers** — routes can be pushed both from a button's `Clickable` handler and from external events (push notification, background sync completing). | |

### 1.3 API sketch

```go
type Route struct {
    Name   string
    Params map[string]string
}

type Router struct {
    stack []Route
}

func (r *Router) Push(route Route)                 // N1, N3
func (r *Router) Pop() (Route, bool)                // N2
func (r *Router) Replace(route Route)               // N2
func (r *Router) Current() Route                    // N2
func (r *Router) ResolveURL(url string) (Route, error) // N4
func (r *Router) Serialize() []byte                 // N5
func (r *Router) Restore(data []byte) error          // N5

// Guard hook, checked before Pop/Push completes.
type NavGuard func(from, to Route) (allow bool)
func (r *Router) SetGuard(g NavGuard)                // N7

// Overlay stack, independent of the main stack, for modals.
func (r *Router) PushModal(route Route)              // N6
func (r *Router) PopModal()                          // N6
```

### 1.4 Example usage

```go
// User taps a row in the incident list.
if row.Clicked() {
    router.Push(Route{
        Name:   "incident.form",
        Params: map[string]string{"id": row.SysID},
    })
}

// The form screen reads its own params.
func IncidentFormScreen(gtx C, route Route) D {
    id := route.Params["id"]
    incident := loadIncident(id)
    // ...render form...
}

// Back button / Android back gesture.
if backPressed {
    router.Pop()
}

// App cold-starts from a push notification.
route, err := router.ResolveURL("gio://app/incident/sys_id_123")
if err == nil {
    router.Push(route)
}

// Unsaved-changes guard.
router.SetGuard(func(from, to Route) bool {
    if from.Name == "incident.form" && formIsDirty {
        return confirmDialog("Discard unsaved changes?")
    }
    return true
})
```

### 1.5 Acceptance criteria

- [ ] Pressing back from a record view returns to the list view with its previous scroll position and active filter preserved.
- [ ] A deep link string can be resolved to a route without any prior interactive navigation.
- [ ] Killing and restoring the process (simulated) returns the user to the same record form.
- [ ] Opening a "New Incident" modal over the list, then cancelling, leaves the list's state untouched.
- [ ] A navigation guard can block a back-navigation and show a confirmation dialog.

### 1.6 Non-goals

- URL bar / browser-style address display (desktop/web only — not required for mobile).
- Animated screen transitions (nice-to-have, not part of this requirement — see also "Animation" as a separate, lower-priority gap).

---

## 2. Data Grid

### 2.1 Reference model

- **Flutter:** `DataTable` (basic, non-virtualized) is insufficient at scale; real apps use `PlutoGrid` or `syncfusion_flutter_datagrid` for virtualization, per-column filtering, and inline editing.
- **ServiceNow:** the **list view** is the primary surface of the product — sortable columns, per-column "condition" filters (contains/starts with/is), saved filters, multi-select for bulk actions, inline (in-cell) editing, related lists embedded inside a form, and personalization (show/hide/reorder columns) persisted per user.

### 2.2 Functional requirements

| # | Requirement | Notes |
|---|---|---|
| G1 | **Virtualized row rendering** — only rows currently in the viewport are laid out/drawn. | Non-negotiable for ServiceNow-scale lists (thousands of rows). Naive Gio widgets that lay out every row will fall apart past a few hundred. |
| G2 | **Column configuration** — id, header label, width (fixed or flexible), alignment, sortable flag, visibility. | Equivalent to ServiceNow's list layout / Flutter `DataColumn`. |
| G3 | **Multi-column sort** — click a header to sort ascending/descending; shift-click (or long-press on mobile) adds a secondary sort key. | |
| G4 | **Per-column filtering** — text contains/equals, numeric range, date range, dropdown-from-choice-list. | ServiceNow's "condition builder" is the gold standard reference; a simplified per-column filter row is sufficient for v1. |
| G5 | **Row selection** — single row (tap to open) and multi-row (checkbox) for bulk actions. | |
| G6 | **Inline cell editing** — a cell can become an editable field in place, with the same validation used by the full form (see Section 3). | Optional for v1; ServiceNow supports this but it's a significant increment in complexity. |
| G7 | **Pagination or infinite scroll** — data source is paged; the grid requests the next page as the user scrolls near the end. | Must work with an async/remote data source, not just an in-memory slice. |
| G8 | **Empty / loading / error states** — explicit states for "no rows," "loading page N," and "fetch failed, retry." | |
| G9 | **Column personalization persistence** — user's column order/width/visibility choices are saved and reapplied. | Lower priority than G1–G5. |
| G10 | **Row action affordances** — swipe-to-reveal actions (mobile) or a trailing icon-button column (desktop-width). | |

### 2.3 API sketch

```go
type Column struct {
    ID        string
    Header    string
    Width     unit.Dp
    Sortable  bool
    Filter    FilterKind // None, Text, Number, DateRange, Choice
}

type SortSpec struct {
    ColumnID   string
    Descending bool
}

type DataSource interface {
    // Fetch returns rows [offset, offset+limit) matching filters/sort.
    Fetch(ctx context.Context, offset, limit int, sort []SortSpec, filters map[string]string) (rows []Row, total int, err error)
}

type Grid struct {
    Columns    []Column
    Source     DataSource
    Selection  map[string]bool // row ID -> selected
    sortState  []SortSpec
    filterState map[string]string
}

func (g *Grid) Layout(gtx C) D                    // G1, virtualized
func (g *Grid) OnHeaderClicked(colID string)      // G3
func (g *Grid) SetFilter(colID, value string)     // G4
func (g *Grid) ToggleSelect(rowID string)         // G5
func (g *Grid) OnScrollNearEnd()                  // G7, triggers next page fetch
```

### 2.4 Example usage

```go
incidentGrid := &Grid{
    Columns: []Column{
        {ID: "number", Header: "Number", Width: unit.Dp(100), Sortable: true},
        {ID: "short_description", Header: "Short description", Width: unit.Dp(300), Sortable: false, Filter: FilterText},
        {ID: "priority", Header: "Priority", Width: unit.Dp(80), Sortable: true, Filter: FilterChoice},
        {ID: "state", Header: "State", Width: unit.Dp(100), Sortable: true, Filter: FilterChoice},
    },
    Source: &IncidentDataSource{}, // implements DataSource against your backend/local DB
}

// User taps the "Priority" column header — sorts descending on second click.
incidentGrid.OnHeaderClicked("priority")

// User types "network" into the short_description filter box.
incidentGrid.SetFilter("short_description", "network")

// User taps a row: open the form via the router (Section 1).
if rowClicked {
    router.Push(Route{Name: "incident.form", Params: map[string]string{"id": row.SysID}})
}
```

### 2.5 Acceptance criteria

- [ ] A 10,000-row data source scrolls smoothly (only visible rows laid out per frame).
- [ ] Clicking a sortable column header re-sorts the visible rows and re-fetches from `DataSource` with the new `SortSpec`.
- [ ] Setting a text filter on a column narrows the result set via `DataSource.Fetch`, not client-side filtering of an already-loaded page.
- [ ] Multi-select checkboxes track selection across a page boundary (scrolling doesn't lose selection state).
- [ ] Scrolling to the bottom triggers a fetch of the next page with a visible loading indicator, and an empty/error state renders correctly when appropriate.

### 2.6 Non-goals

- Full drag-to-reorder columns (nice-to-have, not required for v1).
- Export to CSV/XLSX (can be layered on top of `DataSource` later; not part of the grid widget itself).

---

## 3. Data-Driven Form Abstraction

### 3.1 Reference model

- **Flutter:** `Form` + `FormField` + `FormFieldValidator`, or JSON-schema-driven form libraries (`json_form`, `flutter_form_builder`). Field-level validators run on change/submit; the form tracks overall validity and dirty state.
- **ServiceNow:** the **form** is generated from **dictionary entries** (field type: string/reference/choice/date/boolean/journal, mandatory flag, max length) plus **UI Policies** (declarative rules: "if Category = Hardware, make Configuration Item mandatory and show Serial Number") and **Client Scripts** (imperative on-change logic). Reference fields open a lookup dialog backed by a grid (Section 2) filtered to the target table.

### 3.2 Functional requirements

| # | Requirement | Notes |
|---|---|---|
| F1 | **Field schema** — each field declares: id, label, type, mandatory flag, read-only flag, default value, and (for choice/reference types) its source. | Equivalent to a ServiceNow dictionary entry. |
| F2 | **Field type set** — at minimum: short text, long text (multi-line), number, boolean, date/datetime, choice (fixed list), reference (lookup to another table's grid). | |
| F3 | **Field-level validation** — required, min/max length, regex/pattern, numeric range, custom validator function; validation runs on blur and on submit. | Equivalent to Flutter's `FormFieldValidator<T>`. |
| F4 | **Form-level validity** — the form knows whether *all* fields are currently valid, for enabling/disabling the Save action. | |
| F5 | **Dirty-state tracking** — the form knows whether any field has changed from its loaded value, to drive the navigation guard (Section 1, N7) and a "discard changes" prompt. | |
| F6 | **Conditional visibility / mandatory rules** — a field's visible, mandatory, or read-only state can depend on the current value of other fields. | This is the Gio equivalent of ServiceNow's UI Policies. Should be declarative where possible (a rule table), not just ad-hoc `if` statements scattered through layout code. |
| F7 | **Reference field lookup** — a reference-type field opens a modal grid (Section 2) filtered/searchable against the target table, and selecting a row populates the field with a display value + underlying ID. | |
| F8 | **Submit / cancel lifecycle** — `OnSubmit(values) error` and `OnCancel()` hooks; submit is blocked while the form is invalid (F4) or a submission is already in flight. | |
| F9 | **Schema-driven rendering** — given a field schema slice, the form renders the appropriate widget per field type without hand-written per-field layout code for every screen. | This is what makes the abstraction "data-driven" rather than just "a form" — new record types should mostly be new schemas, not new layout code. |
| F10 | **Server-side / async validation errors** — a submit that fails validation on the backend can attach an error message to the specific field that caused it. | |

### 3.3 API sketch

```go
type FieldType int
const (
    FieldText FieldType = iota
    FieldTextArea
    FieldNumber
    FieldBoolean
    FieldDate
    FieldChoice
    FieldReference
)

type FieldSchema struct {
    ID        string
    Label     string
    Type      FieldType
    Mandatory bool
    ReadOnly  bool
    Choices   []Choice          // for FieldChoice
    RefTable  string            // for FieldReference — which DataSource to look up against
    Validators []Validator
}

type Validator func(value string) (ok bool, message string) // F3

// A UIRule mirrors a ServiceNow UI Policy: a condition over current
// values, and an effect on one or more fields.
type UIRule struct {
    When   func(values map[string]string) bool
    Effect func(field string) (visible, mandatory, readOnly bool)
    Fields []string
}

type Form struct {
    Schema []FieldSchema
    Rules  []UIRule
    values map[string]string
    dirty  map[string]bool
}

func (f *Form) SetValue(fieldID, value string)          // triggers F6 rule re-evaluation
func (f *Form) IsValid() bool                            // F4
func (f *Form) IsDirty() bool                             // F5
func (f *Form) Layout(gtx C) D                            // F9, schema-driven render
func (f *Form) OnSubmit(fn func(values map[string]string) error)
func (f *Form) OnCancel(fn func())
func (f *Form) SetFieldError(fieldID, message string)     // F10
```

### 3.4 Example usage

```go
incidentSchema := []FieldSchema{
    {ID: "short_description", Label: "Short description", Type: FieldText, Mandatory: true,
        Validators: []Validator{maxLength(160)}},
    {ID: "category", Label: "Category", Type: FieldChoice,
        Choices: []Choice{{Value: "hardware", Label: "Hardware"}, {Value: "software", Label: "Software"}}},
    {ID: "configuration_item", Label: "Configuration item", Type: FieldReference, RefTable: "cmdb_ci"},
    {ID: "priority", Label: "Priority", Type: FieldChoice, Mandatory: true},
    {ID: "description", Label: "Description", Type: FieldTextArea},
}

// UI Policy equivalent: Category = Hardware makes Configuration Item mandatory.
rules := []UIRule{
    {
        Fields: []string{"configuration_item"},
        When:   func(v map[string]string) bool { return v["category"] == "hardware" },
        Effect: func(field string) (visible, mandatory, readOnly bool) {
            return true, true, false
        },
    },
}

form := &Form{Schema: incidentSchema, Rules: rules}
form.OnSubmit(func(values map[string]string) error {
    return incidentAPI.Save(values)
})
form.OnCancel(func() { router.Pop() })

// Reference field lookup opens a modal grid (Section 2), filtered to cmdb_ci.
if referenceFieldClicked {
    router.PushModal(Route{
        Name:   "lookup.grid",
        Params: map[string]string{"table": "cmdb_ci", "targetField": "configuration_item"},
    })
}
```

### 3.5 Acceptance criteria

- [ ] Adding a new record type (e.g. "Change Request") requires only a new `[]FieldSchema` + `[]UIRule`, no new hand-written layout code.
- [ ] Setting Category to "Hardware" immediately makes Configuration Item visible and mandatory; switching back to "Software" reverts it.
- [ ] Save is disabled while any mandatory field is empty or any validator fails.
- [ ] Navigating away from a dirty form triggers the navigation guard from Section 1.
- [ ] Tapping a reference field opens a filtered grid lookup and populates the field on selection.
- [ ] A backend validation failure on submit attaches its error message to the correct field, not just a generic toast.

### 3.6 Non-goals

- Imperative "Client Script"-style arbitrary on-change scripting engine — F6's declarative rule table covers the common case; anything more exotic can be handled with a plain Go callback on `SetValue` rather than a full scripting layer.
- Multi-step / wizard forms (out of scope for v1; can be layered on top later as a sequence of `Form`s driven by the router).

---

## 4. How the three compose

```
List screen (Grid, Section 2)
   │ row tapped
   ▼
router.Push("incident.form", {id}) (Router, Section 1)
   │
   ▼
Form screen (Form, Section 3) loads record `id`
   │ reference field tapped
   ▼
router.PushModal("lookup.grid", {table}) (Grid, inside a modal via Router)
   │ row selected in lookup grid
   ▼
router.PopModal(), Form.SetValue(field, selectedID)
   │ user taps Save
   ▼
Form.OnSubmit → API call → on success: router.Pop() back to List screen,
                which re-fetches via DataSource to reflect the change.
```

This is the same shape as a ServiceNow list → form → reference lookup → save → return-to-list flow, and the same shape as a Flutter app built on `go_router` + `Form` + `DataTable`/`PlutoGrid`.

## 5. Suggested build order

1. **Router** first — every other screen depends on it existing, even in a minimal form (push/pop/params, no deep links yet).
2. **Grid** second — needed to view any data at all; start with G1–G5 (virtualization, columns, sort, filter, selection) and defer G6/G9 (inline edit, personalization persistence).
3. **Form** third — build against a *real* schema (e.g. Incident) from day one rather than a toy example, since F6 (conditional rules) is where the design usually needs revisiting once real business rules show up.

Deep links, state restoration, inline grid editing, and column personalization can all be added incrementally once the core loop above is working end-to-end.
