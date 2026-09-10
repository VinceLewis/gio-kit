package dialog

import "github.com/VinceLewis/gio-kit/diagnostic"

func (d *Confirm) DebugSnapshot(diagnostic.Request) diagnostic.Component {
	confirm, cancel := d.ConfirmLabel, d.CancelLabel
	if confirm == "" {
		confirm = "CONFIRM"
	}
	if cancel == "" {
		cancel = "CANCEL"
	}
	return diagnostic.Component{Kind: "dialog", State: map[string]any{"title": d.Title, "message": d.Message, "destructive": d.Destructive, "actions": []string{cancel, confirm}, "open": "unknown"}}
}
