// Package source defines the interface for a data source that can produce items to crawl and accept items that have been crawled.
package source

import "github.com/internetarchive/Zeno/pkg/models"

// Source is an interface for a data source that can produce items to crawl and accept items that have been crawled.
type Source interface {
	// Start initializes the source with the given input and output channels.
	Start(finishChan, produceChan chan *models.Item) error
	// Stop stops the source.
	Stop() error
	// Name returns the name of the source.
	Name() string
}
