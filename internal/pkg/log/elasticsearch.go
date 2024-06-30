package log

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/esapi"
	"github.com/elastic/go-elasticsearch/v8"
)

// ElasticsearchConfig holds the configuration for Elasticsearch logging
type ElasticsearchConfig struct {
	Addresses []string
	Username  string
	Password  string
	Index     string
	Level     slog.Level
}

// ElasticsearchHandler implements slog.Handler for Elasticsearch
type ElasticsearchHandler struct {
	client *elasticsearch.Client
	index  string
	level  slog.Level
	attrs  []slog.Attr
	groups []string
}

// Handle is responsible for passing the log record to all underlying handlers.
// It's called internally when a log message needs to be written.
func (h *ElasticsearchHandler) Handle(ctx context.Context, r slog.Record) error {
	if !h.Enabled(ctx, r.Level) {
		return nil
	}

	doc := make(map[string]interface{})
	doc["timestamp"] = r.Time.Format(time.RFC3339)
	doc["level"] = r.Level.String()
	doc["message"] = r.Message
	doc["attrs"] = make(map[string]interface{})

	// Add pre-defined attributes
	for _, attr := range h.attrs {
		doc["attrs"].(map[string]interface{})[attr.Key] = attr.Value.Any()
	}

	// Add record attributes
	r.Attrs(func(a slog.Attr) bool {
		doc["attrs"].(map[string]interface{})[a.Key] = a.Value.Any()
		return true
	})

	// Handle groups
	if len(h.groups) > 0 {
		current := doc["attrs"].(map[string]interface{})
		for _, group := range h.groups {
			next := make(map[string]interface{})
			current[group] = next
			current = next
		}
	}

	payload, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	req := esapi.IndexRequest{
		Index: h.index,
		Body:  strings.NewReader(string(payload)),
	}

	res, err := req.Do(ctx, h.client)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error indexing document: %s", res.String())
	}

	return nil
}

// Enabled checks if any of the underlying handlers are enabled for a given log level.
// It's used internally to determine if a log message should be processed by a given handler
func (h *ElasticsearchHandler) Enabled(ctx context.Context, level slog.Level) bool {
	_, _ = ctx, level // Ignoring context and level
	return level >= h.level
}

// WithAttrs creates a new handler with additional attributes.
// It's used internally when the logger is asked to include additional context with all subsequent log messages.
func (h *ElasticsearchHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandler := *h
	newHandler.attrs = append(h.attrs, attrs...)
	return &newHandler
}

// WithGroup creates a new handler with a new group added to the attribute grouping hierarchy.
// It's used internally when the logger is asked to group a set of attributes together.
func (h *ElasticsearchHandler) WithGroup(name string) slog.Handler {
	newHandler := *h
	newHandler.groups = append(h.groups, name)
	return &newHandler
}

func (h *ElasticsearchHandler) createIndex() error {
	mapping := `{
		"mappings": {
			"properties": {
				"timestamp": {"type": "date"},
				"level": {"type": "keyword"},
				"message": {"type": "text"},
				"attrs": {"type": "object", "dynamic": true}
			}
		}
	}`

	req := esapi.IndicesCreateRequest{
		Index: h.index,
		Body:  strings.NewReader(mapping),
	}

	res, err := req.Do(context.Background(), h.client)
	if err != nil {
		return fmt.Errorf("error creating index: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		// If the index already exists, that's okay
		if strings.Contains(res.String(), "resource_already_exists_exception") {
			return nil
		}
		return fmt.Errorf("error creating index: %s", res.String())
	}

	return nil
}
