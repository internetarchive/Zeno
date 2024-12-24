package ui

import (
	"fmt"
	"sort"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/internetarchive/Zeno/internal/pkg/controler"
	"github.com/internetarchive/Zeno/internal/pkg/controler/pause"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/stats"
	"github.com/rivo/tview"
)

// UI holds references to the main tview application components.
type UI struct {
	app        *tview.Application
	pages      *tview.Pages
	flex       *tview.Flex
	statsTable *tview.Table
	logsView   *tview.TextView
	controls   *tview.TextView
	logsChan   <-chan string // Read-only channel for logs
}

// New creates and configures the UI. It does not start it yet.
func New() *UI {
	ui := &UI{
		app:        tview.NewApplication(),
		pages:      tview.NewPages(),
		statsTable: tview.NewTable().SetBorders(false).SetSelectable(false, false),
		logsView:   tview.NewTextView(),
		controls:   tview.NewTextView(),
		logsChan:   log.LogChanTUI,
	}

	// Configure the statsTable.
	// You can optionally set a border and title here.
	ui.statsTable.
		SetBorder(true).
		SetTitle(" Stats Table ").
		SetTitleAlign(tview.AlignLeft)

	// Configure the logsView separately to avoid return-type issues.
	ui.logsView.
		SetScrollable(true).
		SetDynamicColors(true)
	ui.logsView.
		Box.SetBorder(true).
		SetTitle(" Logs ")

	// Configure the controls TextView.
	ui.controls.
		SetText("Controls: Press Ctrl+S to manage [Pause|Stop|Cancel]")
	ui.controls.
		Box.SetBorder(true).
		SetTitle(" Info ").
		SetTitleAlign(tview.AlignLeft)

	ui.initLayout()
	ui.initKeybindings()
	return ui
}

// initLayout constructs the main Flex layout: top stats, middle logs, bottom controls.
func (ui *UI) initLayout() {
	// Create the main flex:
	//   - Stats table (expand=3)
	//   - Logs view   (expand=5)
	//   - Controls    (1 row or fixed height)
	mainFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.statsTable, 0, 3, false).
		AddItem(ui.logsView, 0, 5, false).
		AddItem(ui.controls, 1, 0, false)

	ui.flex = mainFlex
	ui.pages.AddPage("main", mainFlex, true, true)
	ui.app.SetRoot(ui.pages, true)
}

// initKeybindings sets up global key handlers, including Ctrl+S to show the modal.
func (ui *UI) initKeybindings() {
	ui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Example: Ctrl+S => Show the modal
		if event.Key() == tcell.KeyCtrlS {
			ui.showModal()
			return nil // Consume the event
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

	// Finally, run the tview application (blocking call).
	return ui.app.Run()
}

// updateStatsLoop periodically fetches stats and updates the UI table.
func (ui *UI) updateStatsLoop() {
	ticker := time.NewTicker(2 * time.Second) // Adjust as needed
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			statMap := stats.GetMap()

			ui.app.QueueUpdateDraw(func() {
				ui.populateStatsTable(statMap)
			})
		}
	}
}

// readLogsLoop continuously reads from logsChan and appends them to the logs view.
func (ui *UI) readLogsLoop() {
	for logMsg := range ui.logsChan {
		// Append log line to logs view.
		ui.app.QueueUpdateDraw(func() {
			fmt.Fprintf(ui.logsView, "%s\n", logMsg)
		})
	}
}

// populateStatsTable updates the statsTable using a wrapping layout
// that places up to three (Key: Value) pairs per row.
func (ui *UI) populateStatsTable(statMap map[string]interface{}) {
	ui.statsTable.Clear()

	// Decide how many columns you want per row:
	const columnsPerRow = 3

	// Gather keys from the map, sort them for consistent display:
	keys := make([]string, 0, len(statMap))
	for k := range statMap {
		keys = append(keys, k)
	}
	sort.Strings(keys) // optional, but nice for stable ordering

	// Fill the table row by row, up to columnsPerRow entries each.
	row := 0
	col := 0
	for _, key := range keys {
		value := statMap[key]
		cellText := fmt.Sprintf("%s: %v", key, value)

		cell := tview.NewTableCell(cellText).
			SetExpansion(1) // So the cell can expand horizontally if needed

		ui.statsTable.SetCell(row, col, cell)

		col++
		if col == columnsPerRow {
			row++
			col = 0
		}
	}
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
				// Do nothing, just close the modal
			}
			// Remove the modal overlay
			ui.pages.RemovePage("modal")
		})

	modal.
		SetBackgroundColor(tcell.ColorDefault).
		SetBorder(true)

	// Add the modal as a new page on top of the main layout
	ui.pages.AddPage("modal", modal, true, true)
}
