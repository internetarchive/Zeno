package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/internetarchive/Zeno/internal/pkg/controler/pause"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/rivo/tview"
)

var menuModalPageName = "menuModal"

func (ui *UI) showMenuModal() {
	isPaused := pause.IsPaused()
	pauseButtonLabel := "Pause"
	if isPaused {
		pauseButtonLabel = "Unpause"
	}

	menuModal := tview.NewModal().
		SetText("Select an action to execute").
		AddButtons([]string{pauseButtonLabel, "Stop", "Cancel"}).
		SetDoneFunc(func(_ int, buttonLabel string) {
			logger := log.NewFieldedLogger(&log.Fields{
				"component": "ui.showMenuModal",
			})

			switch buttonLabel {
			case "Pause", "Unpause":
				if isPaused {
					logger.Info("received unpause action")
					pause.Resume()
				} else {
					logger.Info("received pause action")
					pause.Pause()
				}
			case "Stop":
				logger.Info("received stop action")
				go ui.stop()
				ui.pages.RemovePage(menuModalPageName)
				return
			case "Cancel":
				// do nothing
			}
			ui.pages.RemovePage(menuModalPageName)
		})

	menuModal.SetBackgroundColor(tcell.ColorDefault).SetBorder(true)

	ui.pages.AddPage(menuModalPageName, menuModal, true, true)
}
