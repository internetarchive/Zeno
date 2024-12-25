// ui/ui.go
package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/internetarchive/Zeno/internal/pkg/controler"
	"github.com/internetarchive/Zeno/internal/pkg/controler/pause"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/stats"
	"github.com/rivo/tview"
)

const (
	maxLogLines   = 200 // Keep only the most recent 500 log lines
	columnsPerRow = 3   // How many stats (key-value pairs) per row
)

// UI holds references to the main tview application components.
type UI struct {
	app          *tview.Application
	pages        *tview.Pages
	mainFlex     *tview.Flex // The root vertical flex
	statsRowFlex *tview.Flex // A nested horizontal flex: [left blank] [statsTable] [right blank]

	statsTable *tview.Table
	logsView   *tview.TextView
	controls   *tview.TextView

	logsChan <-chan string // Read-only channel for logs
	logLines []string      // Ring buffer of recent log lines

	screenWidth  int // Terminal width (updated on each draw)
	screenHeight int // Terminal height (updated on each draw)
}

// New creates and configures the UI. It does not start it yet.
func New() *UI {
	// First, create the Table in multiple steps to avoid the "type mismatch" error.
	statsTable := tview.NewTable()
	statsTable.SetBorders(false).
		SetSelectable(false, false)
	// Configure the Box portion separately so it remains a *tview.Table.
	statsTable.Box.SetBorder(true).
		SetTitle(" Stats Table ").
		SetTitleAlign(tview.AlignLeft)

	// Create the TextViews.
	logsView := tview.NewTextView().
		SetScrollable(true).
		SetDynamicColors(true)
	logsView.Box.SetBorder(true).
		SetTitle(" Logs ")

	controls := tview.NewTextView().
		SetText("Controls: Press Ctrl+S (Pause|Stop|Cancel), Ctrl+C to exit")
	controls.Box.SetBorder(true).
		SetTitle(" Info ").
		SetTitleAlign(tview.AlignLeft)

	ui := &UI{
		app:        tview.NewApplication(),
		pages:      tview.NewPages(),
		statsTable: statsTable,
		logsView:   logsView,
		controls:   controls,
		logsChan:   log.LogChanTUI,
		logLines:   make([]string, 0, maxLogLines),
	}

	ui.initLayout()
	ui.initKeybindings()

	return ui
}

// initLayout constructs:
//  1. A horizontal flex (ui.statsRowFlex) to center the stats table:
//     [ blank (10%) | stats table (80%) | blank (10%) ]
//  2. A main vertical flex (ui.mainFlex) stacking:
//     [ statsRowFlex | logsView | controls ]
func (ui *UI) initLayout() {
	// Stats row: center the table
	// We use proportions: 1 : 8 : 1  => 10% | 80% | 10%
	ui.statsRowFlex = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 1, false).           // left blank
		AddItem(ui.statsTable, 0, 8, false). // stats table
		AddItem(nil, 0, 1, false)            // right blank

	// Main vertical layout
	ui.mainFlex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.statsRowFlex, 0, 3, false). // We'll dynamically resize this height
		AddItem(ui.logsView, 0, 5, false).
		AddItem(ui.controls, 1, 0, false)

	ui.pages.AddPage("main", ui.mainFlex, true, true)
	ui.app.SetRoot(ui.pages, true)

	// Capture terminal size on every draw, storing in ui.screenWidth / ui.screenHeight
	ui.app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		w, h := screen.Size()
		ui.screenWidth = w
		ui.screenHeight = h
		return false
	})
}

// initKeybindings sets up global key handlers: Ctrl+S and Ctrl+C.
func (ui *UI) initKeybindings() {
	ui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlS:
			ui.showModal()
			return nil
		case tcell.KeyCtrlC:
			// Graceful shutdown
			controler.Stop()
			ui.app.Stop()
			return nil
		}
		return event
	})
}

// Start launches the TUI event loop and any background goroutines needed.
func (ui *UI) Start() error {
	// Start a goroutine to periodically update stats.
	go ui.updateStatsLoop()

	// Start a goroutine to read logs from the channel.
	go ui.readLogsLoop()

	// Run the tview application (blocking call).
	return ui.app.Run()
}

// updateStatsLoop periodically fetches stats and updates the UI table.
func (ui *UI) updateStatsLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		statMap := stats.GetMap() // map[string]interface{}
		ui.app.QueueUpdateDraw(func() {
			ui.populateStatsTable(statMap)
		})
	}
}

// readLogsLoop continuously reads from logsChan and appends them to the logs view,
// limiting to the most recent maxLogLines.
func (ui *UI) readLogsLoop() {
	for logMsg := range ui.logsChan {
		ui.app.QueueUpdateDraw(func() {
			ui.logLines = append(ui.logLines, logMsg)
			if len(ui.logLines) > maxLogLines {
				ui.logLines = ui.logLines[len(ui.logLines)-maxLogLines:]
			}
			ui.logsView.SetText(strings.Join(ui.logLines, "\n"))
		})
	}
}

// populateStatsTable populates the table with up to columnsPerRow stats per row,
// then dynamically resizes the table height up to half the terminal's height.
func (ui *UI) populateStatsTable(statMap map[string]interface{}) {
	ui.statsTable.Clear()

	// Gather keys from the map and sort them for consistent display.
	keys := make([]string, 0, len(statMap))
	for k := range statMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Fill the table row by row.
	rowCount := 0
	colCount := 0
	for _, key := range keys {
		val := statMap[key]
		cellText := fmt.Sprintf("%s: %v", key, val)

		cell := tview.NewTableCell(cellText).
			SetExpansion(1) // let it expand horizontally if needed

		ui.statsTable.SetCell(rowCount, colCount, cell)
		colCount++
		if colCount == columnsPerRow {
			rowCount++
			colCount = 0
		}
	}
	// If colCount != 0, there's a partially filled row (that's fine).
	totalRowsUsed := rowCount
	if colCount > 0 {
		totalRowsUsed++ // we have a partial row in use
	}

	// Let's account for an extra row or two for borders/titles, etc.
	linesNeeded := totalRowsUsed + 2

	// Limit linesNeeded to half the terminal height
	if ui.screenHeight > 0 {
		halfTerm := ui.screenHeight / 2
		if linesNeeded > halfTerm {
			linesNeeded = halfTerm
		}
	} else {
		// Fallback if we don't have a screen size yet
		linesNeeded = 10
	}

	// Resize the statsRowFlex to a fixed number of lines.
	// We'll do a quick remove/add approach or directly use "ResizeItem" if supported.
	ui.mainFlex.ResizeItem(ui.statsRowFlex, linesNeeded, 0)
}

// showModal creates a modal overlay with [Pause] [Stop] [Cancel].
func (ui *UI) showModal() {
	modal := tview.NewModal().
		SetText("Select an action to execute").
		AddButtons([]string{"Pause", "Stop", "Cancel"}).
		SetDoneFunc(func(_ int, buttonLabel string) {
			switch buttonLabel {
			case "Pause":
				pause.Pause()
			case "Stop":
				controler.Stop()
				ui.app.Stop()
			case "Cancel":
				// No action
			}
			ui.pages.RemovePage("modal")
		})

	modal.SetBackgroundColor(tcell.ColorDefault).
		SetBorder(true)

	ui.pages.AddPage("modal", modal, true, true)
}
