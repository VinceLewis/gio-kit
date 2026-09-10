package guitest_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/VinceLewis/gio-kit/guitest"
)

type platformClipboardAdapter struct {
	read  func(context.Context) (string, error)
	write func(context.Context, string, []byte) error
}

func (a platformClipboardAdapter) ReadClipboard(ctx context.Context) (string, error) {
	return a.read(ctx)
}

func (a platformClipboardAdapter) WriteClipboard(ctx context.Context, mime string, data []byte) error {
	if a.write == nil {
		return nil
	}
	return a.write(ctx, mime, data)
}

func TestPlatformClipboardReadOutcomesUseGioPastePath(t *testing.T) {
	for _, tc := range []struct {
		name    string
		adapter guitest.ClipboardAdapter
		limit   int
		want    string
		wantErr error
	}{
		{name: "unavailable", limit: 32, want: "seed", wantErr: guitest.ErrClipboardUnavailable},
		{name: "denied", limit: 32, want: "seed", wantErr: guitest.ErrClipboardDenied, adapter: platformClipboardAdapter{read: func(context.Context) (string, error) {
			return "", guitest.ErrClipboardDenied
		}}},
		{name: "empty", limit: 32, want: "seed", adapter: platformClipboardAdapter{read: func(context.Context) (string, error) {
			return "", nil
		}}},
		{name: "unicode", limit: 64, want: "seed città 🌍", adapter: platformClipboardAdapter{read: func(context.Context) (string, error) {
			return " città 🌍", nil
		}}},
		{name: "bounded", limit: 4, want: "seed", wantErr: guitest.ErrClipboardTooLarge, adapter: platformClipboardAdapter{read: func(context.Context) (string, error) {
			return "12345", nil
		}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var editor widget.Editor
			var reads atomic.Int32
			adapter := tc.adapter
			if concrete, ok := adapter.(platformClipboardAdapter); ok {
				read := concrete.read
				concrete.read = func(ctx context.Context) (string, error) {
					defer reads.Add(1)
					return read(ctx)
				}
				adapter = concrete
			}
			th := theme()
			d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
				return material.Editor(th, &editor, "Paste target").Layout(gtx)
			}, guitest.WithClipboard(adapter, tc.limit))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = d.Close() })
			if err := d.Type(guitest.Role(semantic.Editor), "seed"); err != nil {
				t.Fatal(err)
			}
			if err := d.SetSelection(key.Range{Start: 4, End: 4}); err != nil {
				t.Fatal(err)
			}
			if err := d.Key("V", key.ModShortcut); err != nil {
				t.Fatal(err)
			}
			if err := d.WaitFor(testContext(t), func() bool {
				if tc.wantErr != nil {
					return errors.Is(d.ClipboardError(), tc.wantErr)
				}
				return reads.Load() == 1 && editor.Text() == tc.want
			}); err != nil {
				t.Fatal(err)
			}
			if got := editor.Text(); got != tc.want {
				t.Fatalf("text = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPlatformClipboardWriteUsesGioCopyPath(t *testing.T) {
	var editor widget.Editor
	type write struct {
		mime string
		text string
	}
	writes := make(chan write, 1)
	adapter := platformClipboardAdapter{
		read: func(context.Context) (string, error) { return "", nil },
		write: func(_ context.Context, mime string, data []byte) error {
			writes <- write{mime: mime, text: string(data)}
			return nil
		},
	}
	th := theme()
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		return material.Editor(th, &editor, "Copy source").Layout(gtx)
	}, guitest.WithClipboard(adapter, 64))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Type(guitest.Role(semantic.Editor), "città 🌍"); err != nil {
		t.Fatal(err)
	}
	if err := d.Key("A", key.ModShortcut); err != nil {
		t.Fatal(err)
	}
	if err := d.Key("C", key.ModShortcut); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-writes:
		if got.mime != "application/text" || got.text != "città 🌍" {
			t.Fatalf("write = %#v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("clipboard write did not complete")
	}
}

func TestPlatformClipboardDropsStaleAsyncRead(t *testing.T) {
	started := make(chan int, 2)
	done := make(chan int, 2)
	oldRelease := make(chan struct{})
	newRelease := make(chan struct{})
	var calls atomic.Int32
	adapter := platformClipboardAdapter{
		read: func(context.Context) (string, error) {
			call := int(calls.Add(1))
			started <- call
			if call == 1 {
				<-oldRelease // Deliberately emulate a platform call slow to cancel.
				done <- call
				return "stale", nil
			}
			<-newRelease
			done <- call
			return "latest", nil
		},
	}
	var first, second widget.Editor
	showSecond := false
	th := theme()
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		if showSecond {
			return material.Editor(th, &second, "Second").Layout(gtx)
		}
		return material.Editor(th, &first, "First").Layout(gtx)
	}, guitest.WithClipboard(adapter, 64))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		select {
		case <-oldRelease:
		default:
			close(oldRelease)
		}
		_ = d.Close()
	})
	if err := d.Focus(guitest.Role(semantic.Editor)); err != nil {
		t.Fatal(err)
	}
	if err := d.Key("V", key.ModShortcut); err != nil {
		t.Fatal(err)
	}
	waitClipboardCall(t, started, 1)
	showSecond = true
	if err := d.Frame(); err != nil {
		t.Fatal(err)
	}
	if err := d.Focus(guitest.Role(semantic.Editor)); err != nil {
		t.Fatal(err)
	}
	if err := d.Key("V", key.ModShortcut); err != nil {
		t.Fatal(err)
	}
	waitClipboardCall(t, started, 2)
	close(newRelease)
	if err := d.WaitFor(testContext(t), func() bool { return second.Text() == "latest" }); err != nil {
		t.Fatal(err)
	}
	waitClipboardCall(t, done, 2)
	close(oldRelease)
	waitClipboardCall(t, done, 1)
	if err := d.Frame(); err != nil {
		t.Fatal(err)
	}
	if got := second.Text(); got != "latest" {
		t.Fatalf("stale read applied: %q", got)
	}
}

func waitClipboardCall(t *testing.T, calls <-chan int, want int) {
	t.Helper()
	select {
	case got := <-calls:
		if got != want {
			t.Fatalf("clipboard call = %d, want %d", got, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("clipboard call %d did not start", want)
	}
}
