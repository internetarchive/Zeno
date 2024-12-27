package ui

import (
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/controler/pause"
	"github.com/rivo/tview"
)

func (ui *UI) pauseMonitor() {
	ui.wg.Add(1)
	go func() {
		defer ui.wg.Done()

		ticker := time.NewTicker(300 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ui.ctx.Done():
				return // We’re shutting down
			case <-ticker.C:
				currentPaused := pause.IsPaused()

				// Check if the menu modal is open:
				menuOpen := ui.pages.HasPage(menuModalPageName)

				// We do all UI changes in the TUI event loop:
				ui.app.QueueUpdateDraw(func() {
					switch {
					case currentPaused && !menuOpen:
						// If we *are* paused and *no* menu is open => show paused modal
						ui.showPausedModal()
					default:
						// Either not paused, or the menu is open => hide paused modal
						ui.hidePausedModal()
					}
				})
			}
		}
	}()
}

func (ui *UI) showPausedModal() {
	// If it's already added, do nothing.
	if ui.pages.HasPage("pausedModal") {
		return
	}
	pausedModal := tview.NewModal().SetText("PAUSED")

	// We add it as a *page*, so it overlays everything else
	ui.pages.AddPage("pausedModal", pausedModal, true, true)
}

func (ui *UI) hidePausedModal() {
	if ui.pages.HasPage("pausedModal") {
		ui.pages.RemovePage("pausedModal")
	}
}
