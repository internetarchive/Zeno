// ui/ui.go
package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/internetarchive/Zeno/internal/pkg/controler"
	"github.com/internetarchive/Zeno/internal/pkg/controler/pause"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/stats"
	"github.com/rivo/tview"
)

const (
	columnsPerRow = 3 // How many (key: value) stats per row
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
	logLines []string      // The lines currently in memory (trimmed dynamically)

	screenWidth  int // Terminal width (updated on each draw)
	screenHeight int // Terminal height (updated on each draw)

	// sync primitives
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates and configures the UI. It does not start it yet.
func New() *UI {
	// First, create the Table in multiple steps to avoid type-mismatch.
	statsTable := tview.NewTable().
		SetBorders(false).
		SetSelectable(false, false)
	statsTable.Box.SetBorder(true).
		SetTitle(" Stats Table ")

	// Logs view.
	logsView := tview.NewTextView().
		SetScrollable(true).
		SetDynamicColors(true)
	logsView.Box.SetBorder(true).
		SetTitle(" Logs ")

	// Controls text (make it multiline-friendly).
	controls := tview.NewTextView().
		SetDynamicColors(true). // enable color tags
		SetText("[::r]CTRL+S: OPEN MENU — CTRL+C: EXIT[-::-]").
		SetTextAlign(tview.AlignCenter)

	// Here we turn off the border:
	controls.Box.SetBorder(false)

	ctx, cancel := context.WithCancel(context.Background())

	ui := &UI{
		app:        tview.NewApplication(),
		pages:      tview.NewPages(),
		statsTable: statsTable,
		logsView:   logsView,
		controls:   controls,
		logsChan:   log.LogChanTUI,
		logLines:   make([]string, 0),
		wg:         sync.WaitGroup{},
		ctx:        ctx,
		cancel:     cancel,
	}

	ui.initLayout()
	ui.initKeybindings()

	return ui
}

// initLayout constructs:
//  1. A horizontal flex (statsRowFlex) to center the stats table.
//  2. A main vertical flex (mainFlex) stacking: [ statsRowFlex | logsView | controls ].
func (ui *UI) initLayout() {
	// Stats row: 1:8:1 proportion to center the stats table.
	ui.statsRowFlex = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 1, false).           // left blank
		AddItem(ui.statsTable, 0, 8, false). // stats table
		AddItem(nil, 0, 1, false)            // right blank

	// Main vertical layout:
	//   - statsRowFlex at the top
	//   - logsView in the middle
	//   - controls at the bottom (with a height of 2 lines)
	ui.mainFlex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.statsRowFlex, 0, 3, false). // We'll resize this height dynamically
		AddItem(ui.logsView, 0, 5, false).
		AddItem(ui.controls, 1, 0, false) // 2 lines for the controls area

	ui.pages.AddPage("main", ui.mainFlex, true, true)
	ui.app.SetRoot(ui.pages, true)

	// Capture terminal size on each draw:
	ui.app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		w, h := screen.Size()
		ui.screenWidth = w
		ui.screenHeight = h
		return false
	})
}

// initKeybindings sets up global key handlers: Ctrl+S, Ctrl+C.
func (ui *UI) initKeybindings() {
	ui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		logger := log.NewFieldedLogger(&log.Fields{
			"component": "ui.InputCapture",
		})
		switch event.Key() {
		case tcell.KeyCtrlS:
			ui.showModal()
			return nil
		case tcell.KeyCtrlC:
			// Graceful shutdown in a separate goroutine
			logger.Info("received CTRL+C signal, stopping services...")
			go ui.stop()
			return nil
		}
		return event
	})
}

// Start launches the TUI event loop and background goroutines.
func (ui *UI) Start() error {
	go ui.updateStatsLoop()
	go ui.readLogsLoop()

	// Run tview (blocking).
	return ui.app.Run()
}

// updateStatsLoop periodically fetches stats & updates the table.
func (ui *UI) updateStatsLoop() {
	ui.wg.Add(1)
	defer ui.wg.Done()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ui.ctx.Done():
			return
		case <-ticker.C:
			statMap := stats.GetMap() // get your stats map
			ui.app.QueueUpdateDraw(func() {
				ui.populateStatsTable(statMap)
			})
		}
	}
}

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

// populateStatsTable lays out the stats in columnsPerRow, then resizes the box.
func (ui *UI) populateStatsTable(statMap map[string]interface{}) {
	ui.statsTable.Clear()

	// Sort keys for stable order
	keys := make([]string, 0, len(statMap))
	for k := range statMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Fill table cells in columns of columnsPerRow
	rowCount := 0
	colCount := 0
	for _, key := range keys {
		val := statMap[key]
		cellText := fmt.Sprintf("%s: %v", key, val)

		cell := tview.NewTableCell(cellText).
			SetExpansion(1)
		ui.statsTable.SetCell(rowCount, colCount, cell)

		colCount++
		if colCount == columnsPerRow {
			rowCount++
			colCount = 0
		}
	}
	if colCount > 0 {
		rowCount++ // partial row
	}

	// linesNeeded = #rows + 2 for borders, etc.
	linesNeeded := rowCount + 2
	if ui.screenHeight > 0 {
		halfTerm := ui.screenHeight / 2
		if linesNeeded > halfTerm {
			linesNeeded = halfTerm
		}
	} else {
		linesNeeded = 10
	}
	ui.mainFlex.ResizeItem(ui.statsRowFlex, linesNeeded, 0)
}

// showModal creates a modal overlay with [Pause] [Stop] [Cancel].
func (ui *UI) showModal() {
	modal := tview.NewModal().
		SetText("Select an action to execute").
		AddButtons([]string{"Pause", "Stop", "Cancel"}).
		SetDoneFunc(func(_ int, buttonLabel string) {
			logger := log.NewFieldedLogger(&log.Fields{
				"component": "ui.showModal",
			})
			switch buttonLabel {
			case "Pause":
				logger.Info("received pause action")
				pause.Pause()
			case "Stop":
				logger.Info("received stop action")
				go ui.stop()
				ui.pages.RemovePage("modal")
				return
			case "Cancel":
				// do nothing
			}
			ui.pages.RemovePage("modal")
		})

	modal.SetBackgroundColor(tcell.ColorDefault).
		SetBorder(true)

	ui.pages.AddPage("modal", modal, true, true)
}

// stop is called from Ctrl+C or Stop button: stops pipeline + TUI.
func (ui *UI) stop() {
	// Show a "Stopping..." modal
	modal := tview.NewModal().SetText("Stopping...")
	ui.pages.AddPage("stopping", modal, true, true)
	ui.app.Draw()

	// Stop the pipeline
	controler.Stop()

	// Cancel all UI loops
	ui.cancel()
	ui.wg.Wait()

	// Stop the Tview app
	ui.app.Stop()
}
