package dumper

import (
	"fmt"
	"os"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/reactor"
	"github.com/internetarchive/Zeno/pkg/models"
)

// Dump writes a spew dump of the items and an ASCII pretty print of the items to a dump file then returns the path to the dump file.
func Dump(items ...*models.Item) string {
	// Creates a dump file to be written to by the dumper
	var dumpFilePath string
	if dumpFilePath = config.Get().LogFileOutputDir; dumpFilePath == "" {
		dumpFilePath = fmt.Sprintf("%s/logs/dump-%s", config.Get().JobPath, time.Now().Format(time.RFC3339))
	} else {
		dumpFilePath = fmt.Sprintf("%s/dump-%s", dumpFilePath, time.Now().Format(time.RFC3339))
	}
	dumpFile, err := os.Create(dumpFilePath)
	if err != nil {
		log.Error("failed to create dump file: %v", err)
	}
	defer dumpFile.Close()

	if len(items) == 0 {
		items = reactor.GetStateTableItems()
	}

	for i := range items {
		fmt.Fprintf(dumpFile, "Item: %s\n", items[i].GetID())
		spew.Fdump(dumpFile, items[i])
		fmt.Fprintf(dumpFile, "\n%s\n_______________________________", items[i].DrawTreeWithStatus())
	}

	return dumpFilePath
}

// PanicWithDump writes a spew dump of the items and an ASCII pretty print of the items to a dump file then panics with a message.
func PanicWithDump(message string, items ...*models.Item) {
	dumpFilePath := Dump(items...)
	panic(fmt.Sprintf("with item dump file: %s AND message: %s", dumpFilePath, message))
}
