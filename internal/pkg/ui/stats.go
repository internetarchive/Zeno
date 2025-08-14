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
			statMap := stats.GetMapTUI()
			ui.app.QueueUpdateDraw(func() {
				ui.populateStatsTable(statMap)
			})
		}
	}
}

// populateStatsTable lays out the stats in columnsPerRow, then resizes the box.
func (ui *UI) populateStatsTable(statMap map[string]any) {
	ui.statsTable.Clear()

	displayMap := flattenStatsMap(statMap)

	// Sort keys for stable order
	keys := make([]string, 0, len(displayMap))
	for k := range displayMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Fill table cells in columns of columnsPerRow
	rowCount := 0
	colCount := 0
	for _, key := range keys {
		val := displayMap[key]
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

func flattenStatsMap(statMap map[string]any) map[string]any {
	flatMap := make(map[string]any)

	for primaryKey, primaryValue := range statMap {
		switch statMap[primaryKey].(type) {
		case map[string]uint64:
			subMap := statMap[primaryKey].(map[string]uint64)
			for subKey, subValue := range subMap {
				flatMap[fmt.Sprintf("%s.%s", primaryKey, subKey)] = subValue
			}
		default:
			flatMap[primaryKey] = primaryValue
		}
	}

	return flatMap
}
