package ui

import (
	"strings"
	"time"
)

// readLogsLoop continuously reads from logsBuffer and updates the logs view.
func (ui *UI) readLogsLoop() {
	ui.wg.Add(1)
	defer ui.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ui.ctx.Done():
			return
		case <-ticker.C:
			ui.app.QueueUpdateDraw(func() {
				// 1) Compute how many lines logsView can display:
				//    logsView.GetInnerRect() => (x, y, width, height)
				_, _, _, logHeight := ui.logsView.GetInnerRect()
				if logHeight < 1 {
					logHeight = 1 // fallback, just in case
				} else {
					logHeight -= 2 // reserve space for the border
				}

				// 2) Get the last logHeight lines from logsBuffer
				newLogLines := ui.logsBuffer.DumpN(uint64(logHeight))

				// 2.1) Append new log lines to the existing ones
				ui.logLines = append(ui.logLines, newLogLines...)

				// 2.2) Trim the logLines to the last logHeight lines
				if len(ui.logLines) > logHeight {
					ui.logLines = ui.logLines[len(ui.logLines)-logHeight:]
				}

				// 3) Update display
				ui.logsView.SetText(strings.Join(ui.logLines, "\n"))
			})
		}
	}
}
