package ui

import (
	"fmt"
	"sort"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/stats"
	"github.com/rivo/tview"
)

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
