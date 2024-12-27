package ui

import "strings"

// readLogsLoop continuously reads from logsChan and appends them to the logs view.
func (ui *UI) readLogsLoop() {
	ui.wg.Add(1)
	defer ui.wg.Done()

	for {
		select {
		case <-ui.ctx.Done():
			return
		case logMsg, ok := <-ui.logsChan:
			if !ok {
				return
			}
			ui.app.QueueUpdateDraw(func() {
				// 1) Append
				ui.logLines = append(ui.logLines, logMsg)

				// 2) Compute how many lines logsView can display:
				//    logsView.GetInnerRect() => (x, y, width, height)
				_, _, _, logHeight := ui.logsView.GetInnerRect()
				if logHeight < 1 {
					logHeight = 1 // fallback, just in case
				}

				// 3) Trim to that many lines
				if len(ui.logLines) > logHeight {
					ui.logLines = ui.logLines[len(ui.logLines)-logHeight:]
				}

				// 4) Update display
				ui.logsView.SetText(strings.Join(ui.logLines, "\n"))
			})
		}
	}
}
