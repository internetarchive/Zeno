// Package ui provides the terminal user interface for Zeno.
package ui

import (
	"context"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/internetarchive/Zeno/internal/pkg/controler"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/log/ringbuffer"
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

	logsBuffer *ringbuffer.MP1COverwritingRingBuffer[string]
	logLines   []string // The lines currently in memory (trimmed dynamically)

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
		SetText("[::r]M: OPEN MENU — CTRL+C: EXIT[-::-]").
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
		logsBuffer: log.TUIRingBuffer,
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

// initKeybindings sets up global key handlers: M, Ctrl+C.
func (ui *UI) initKeybindings() {
	ui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		logger := log.NewFieldedLogger(&log.Fields{
			"component": "ui.InputCapture",
		})

		switch event.Key() {
		case tcell.KeyRune:
			if event.Rune() == 'M' || event.Rune() == 'm' {
				ui.showMenuModal()
				return nil
			}
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
	go ui.pauseMonitor()

	// Run tview (blocking).
	return ui.app.Run()
}

// stop is called from Ctrl+C or Stop button: stops pipeline + TUI.
func (ui *UI) stop() {
	// Show a "Stopping..." modal
	stoppingModal := tview.NewModal().SetText("Stopping...")
	ui.pages.AddPage("stoppingModal", stoppingModal, true, true)
	ui.app.Draw()

	// Stop the pipeline
	controler.Stop()

	// Cancel all UI loops
	ui.cancel()
	ui.wg.Wait()

	// Stop the Tview app
	ui.app.Stop()
}
