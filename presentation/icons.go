package presentation

import (
	"strings"

	"gioui.org/widget"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

// SemanticIcon resolves generic semantic names to Material vector icons.
// An unsupported name returns nil: callers retain the declared text label.
// Applications resolve their own icon maps before calling this function.
func SemanticIcon(name string) *widget.Icon {
	var data []byte
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), "_", "-")) {
	case "home":
		data = icons.ActionHome
	case "dashboard":
		data = icons.ActionDashboard
	case "calendar", "calendar-days", "date-range":
		data = icons.ActionDateRange
	case "event", "calendar-event":
		data = icons.ActionEvent
	case "person", "user", "account", "account-circle":
		data = icons.ActionAccountCircle
	case "people", "users", "group":
		data = icons.SocialPeople
	case "music", "music-note":
		data = icons.ImageMusicNote
	case "mic", "microphone":
		data = icons.AVMic
	case "place", "location", "location-on", "map-pin":
		data = icons.MapsPlace
	case "time", "clock", "schedule":
		data = icons.ActionSchedule
	case "check", "done":
		data = icons.ActionDone
	case "check-circle":
		data = icons.ActionCheckCircle
	case "close", "clear", "x":
		data = icons.NavigationClose
	case "block", "ban", "prohibited":
		data = icons.ContentBlock
	case "warning", "alert", "alert-triangle":
		data = icons.AlertWarning
	case "error", "alert-circle":
		data = icons.AlertError
	case "info", "information":
		data = icons.ActionInfo
	case "help", "help-circle":
		data = icons.ActionHelpOutline
	case "add", "plus":
		data = icons.ContentAdd
	case "edit", "pencil":
		data = icons.ImageEdit
	case "delete", "trash":
		data = icons.ActionDelete
	case "save":
		data = icons.ContentSave
	case "search":
		data = icons.ActionSearch
	case "settings", "gear":
		data = icons.ActionSettings
	case "list":
		data = icons.ActionList
	case "menu":
		data = icons.NavigationMenu
	case "more", "more-vert":
		data = icons.NavigationMoreVert
	case "refresh", "reload":
		data = icons.NavigationRefresh
	case "sync":
		data = icons.NotificationSync
	case "log-out", "logout":
		data = icons.ActionExitToApp
	case "dot":
		data = icons.ImageLens
	case "back", "arrow-back":
		data = icons.NavigationArrowBack
	case "forward", "arrow-forward":
		data = icons.NavigationArrowForward
	case "chevron-left", "previous":
		data = icons.NavigationChevronLeft
	case "chevron-right", "next":
		data = icons.NavigationChevronRight
	default:
		return nil
	}
	icon, err := widget.NewIcon(data)
	if err != nil {
		return nil
	}
	return icon
}
