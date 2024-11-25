package log

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/elastic/go-elasticsearch/esapi"
	elastic "github.com/elastic/go-elasticsearch/v7"
)

// ElasticsearchDestination logs to Elasticsearch
type ElasticsearchDestination struct {
	level     slog.Level
	config    *ElasticsearchConfig
	client    *elastic.Client
	index     string
	mu        sync.Mutex
	ticker    *time.Ticker
	closeChan chan struct{}
}

func NewElasticsearchDestination() *ElasticsearchDestination {
	// parse comma-separated list of Elasticsearch addresses
	// e.g. "http://localhost:9200,http://localhost:9201"
	addresses := strings.Split(globalConfig.ElasticsearchConfig.Addresses, ",")
	if len(addresses) == 0 {
		return nil
	}
	es, err := elastic.NewClient(elastic.Config{
		Addresses: addresses,
		Username:  globalConfig.ElasticsearchConfig.Username,
		Password:  globalConfig.ElasticsearchConfig.Password,
	})
	if err != nil {
		// Handle error (for simplicity, we'll just ignore it here)
		return nil
	}

	ed := &ElasticsearchDestination{
		level:     globalConfig.ElasticsearchConfig.Level,
		config:    globalConfig.ElasticsearchConfig,
		client:    es,
		index:     globalConfig.ElasticsearchConfig.IndexPrefix + "-" + time.Now().Format("2006.01.02"),
		closeChan: make(chan struct{}),
	}

	if globalConfig.ElasticsearchConfig.Rotate && globalConfig.ElasticsearchConfig.RotatePeriod > 0 {
		ed.ticker = time.NewTicker(globalConfig.ElasticsearchConfig.RotatePeriod)
		go ed.rotationWorker()
	}

	return ed
}

func (d *ElasticsearchDestination) Enabled() bool {
	return d.client != nil
}

func (d *ElasticsearchDestination) Level() slog.Level {
	return d.level
}

func (d *ElasticsearchDestination) Write(entry *logEntry) {
	doc := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"level":     entry.level.String(),
		"message":   entry.msg,
		"fields":    entry.args,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(doc); err != nil {
		// Handle error
		return
	}

	req := esapi.IndexRequest{
		Index:      d.index,
		DocumentID: "", // Auto-generate ID
		Body:       &buf,
		Refresh:    "true",
	}

	_, err := req.Do(context.Background(), d.client)
	if err != nil {
		// Handle error
	}
}

func (d *ElasticsearchDestination) Close() {
	if d.ticker != nil {
		d.ticker.Stop()
	}
	close(d.closeChan)
}

func (d *ElasticsearchDestination) rotationWorker() {
	for {
		select {
		case <-d.ticker.C:
			d.mu.Lock()
			d.index = d.config.IndexPrefix + "-" + time.Now().Format("2006.01.02")
			d.mu.Unlock()
		case <-d.closeChan:
			return
		}
	}
}
