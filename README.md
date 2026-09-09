# Gio Kit

Reusable navigation, virtualized data-grid, and metadata-driven form packages
for [Gio](https://gioui.org/) CRUD applications. Applications retain ownership
of their window, theme, records, database, and backend.

The repository also includes an Android integration harness with a 10,000-row
SQLite incident dataset.

## Install

```sh
go get github.com/VinceLewis/gio-kit
```

```go
import (
    "github.com/VinceLewis/gio-kit/form"
    "github.com/VinceLewis/gio-kit/grid"
    gridsqlite "github.com/VinceLewis/gio-kit/grid/sqlite"
    "github.com/VinceLewis/gio-kit/router"
)
```

The current Gio baseline is `gioui.org v0.10.2`.

The window-free testing framework is being implemented for local Termux use.
See the [developer/LLM guide](docs/guitest.md) and
[delivery progress](test-framework-progress.md) for available APIs and pending
device gates.

## Data grid

The grid controller requests asynchronous pages from a `DataSource`; it never
loads the complete dataset. The SQLite adapter builds bound filtering, sorting,
counting, and paging queries.

```go
source, err := gridsqlite.New(db, gridsqlite.Config{
    Table:    "incident",
    IDColumn: "sys_id",
    Columns: map[string]string{
        "number":      "number",
        "description": "short_description",
        "priority":    "priority",
    },
})
if err != nil {
    log.Fatal(err)
}

controller, err := grid.NewController([]grid.Column{
    {
        ID: "number", Header: "Number",
        Width: 110, Visible: true, Sortable: true,
        Filter: grid.FilterText,
    },
    {
        ID: "description", Header: "Description",
        Flex: 2, Visible: true, Filter: grid.FilterText,
    },
    {
        ID: "priority", Header: "Priority",
        Width: 80, Visible: true, Sortable: true,
        Filter: grid.FilterNumber,
    },
}, source, 50, window.Invalidate)
if err != nil {
    log.Fatal(err)
}
defer controller.Close()

table := grid.NewWidget(controller)
table.OpenColumn = "number"
table.CardTitleColumn = "number"
table.CardSummaryColumn = "description"
table.OnRow = func(row grid.Row) {
    log.Println("open record", row.ID)
}

if err := controller.Refresh(); err != nil {
    log.Fatal(err)
}
```

Retain the controller and widget outside frame layout, then draw the widget:

```go
table.Layout(gtx, theme)
```

Current interactions:

- A single tap on the configured `OpenColumn` opens the row.
- Sortable headers toggle ascending and descending order.
- Rows use checkboxes for selection.
- The Select header checkbox selects or clears all currently loaded rows.
- Selection is keyed by row ID and survives paging.
- `ViewAuto` (the default) renders cards below `CardBreakpoint` (600dp) and a
  table at wider sizes; set `ViewMode` to `ViewCards` or `ViewTable` for a user
  override.
- Narrow forced tables scroll horizontally with one shared header/body offset.
- Card and table presentations share the same virtual list, query, sort,
  selection, paging, retry, and row-open callbacks.
- `EnableSelection = false` hides selection for lookup-only grids.
- Column order, width, and visibility preferences are serializable.
- Loading, empty, error/retry, cancellation, and stale-result states are
  handled explicitly.

Filters run through the data source rather than against a partial page:

```go
err := controller.SetFilter("description", grid.Filter{
    Operator: grid.Contains,
    Value:    "network",
})
```

## Data-driven forms

```go
incidentForm, err := form.New([]form.FieldSchema{
    {
        ID: "short_description", Label: "Short description",
        Type: form.FieldText, Mandatory: true,
        Validators: []form.Validator{form.MaxLength(160)},
    },
    {
        ID: "category", Label: "Category",
        Type: form.FieldChoice,
        Choices: []form.Choice{
            {Value: "software", Label: "Software"},
            {Value: "hardware", Label: "Hardware"},
        },
    },
    {
        ID: "configuration_item", Label: "Configuration item",
        Type: form.FieldReference, RefTable: "incident",
    },
}, rules, window.Invalidate)
if err != nil {
    log.Fatal(err)
}

formWidget := form.NewWidget(incidentForm)
formWidget.OnReference = func(field form.FieldSchema) {
    // Open a lookup modal, then call SetReference with the selected row.
}
formWidget.OnInvalid = func() {
    // Display a "Fix errors before saving" message.
}
```

Current interactions:

- Choice fields open a conventional value list instead of cycling values.
- The form scrolls an opened choice field into view.
- Choice lists show at most five rows and scroll when more values exist.
- A long press selects the touched word and opens Select All, Cut, Copy, and
  Paste controls.
- Cut, Copy, and Paste dismiss the text action menu.
- Mandatory and custom validation errors attach to their fields.
- Invalid save attempts invoke `OnInvalid`.
- Save remains on the record; Save & Close invokes `OnSaveAndClose`.
- Reference lookups can use a modal grid without triggering the dirty-form
  navigation guard.
- Backend `form.FieldErrors` attach messages to exact fields.

Gio v0.10.2 does not expose Android's native text-selection ActionMode, so the
text action menu is implemented with Gio clipboard commands.

## Navigation

The router supports typed route parameters, deep-link resolution, push/pop
history, modal routes, dirty-state guards, and versioned serialization for
process restoration.

```go
routes, err := router.NewTable(
    router.Definition{Name: "incident.list", Pattern: "/incidents"},
    router.Definition{Name: "incident.form", Pattern: "/incident/:id"},
)
nav, err := router.New(routes, router.Route{Name: "incident.list"})

err = nav.Push(router.Route{
    Name:   "incident.form",
    Params: router.Params{"id": router.String("id-1")},
})
```

## Test

Full default-package tests and vet on this Termux phone:

```sh
./tools/test-termux.sh -count=1
```

The script uses the existing pinned NDK's Vulkan/EGL headers and Android
libraries with command-local compiler settings; no global configuration or
toolchain upgrade is needed. Some NDK/compiler warnings remain. Focused checks
such as `go test ./router ./grid ./grid/sqlite ./form` still work independently.
Run the separate [framework checks](docs/guitest.md#run-on-this-phone) for
GPU-isolation checks and the tagged demo tests. None replaces manual APK testing.

## Android demo

The build script validates the pinned Termux Android toolchain, produces a
signed arm64 APK, verifies its signature, and checks that `libgio.so` links
Android `libEGL.so`.

```sh
./tools/build-form-apk.sh
```

The generated APK is copied to
`/storage/emulated/0/Download/gio-kit-form-phase3-release.apk` and is excluded
from Git.

## License and credits

Gio Kit is available under the [BSD 3-Clause License](LICENSE), including for
commercial use. Preserve the copyright and license notices when redistributing
it.

Project credit: [github.com/VinceLewis/gio-kit](https://github.com/VinceLewis/gio-kit).
See [NOTICE](NOTICE) for attribution information. Third-party components retain
their respective licenses.
