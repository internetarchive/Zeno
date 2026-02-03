package frontier

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/reactor"
	"github.com/internetarchive/Zeno/pkg/models"
)

// Representation of models.Item sent over WebSocket.
type SerializedItem struct {
	ID      string `json:"id"`
	URL     string `json:"url"`
	SeedVia string `json:"seedVia"`
	Status  string `json:"status"`
	Source  string `json:"source"`
	Parent  string `json:"parent"` // children are ommited, because they would trigger lots of updates, parent is enough to track the tree
	Err     string `json:"err,omitempty"`
}

// Converts a models.Item to a SerializedItem.
func serializedFromItem(item *models.Item) SerializedItem {
	s := SerializedItem{
		ID:      item.GetID(),
		SeedVia: item.GetSeedVia(),
		Status:  item.GetStatus().String(),
		Source:  item.GetSource().String(),
	}
	if item.GetURL() != nil {
		s.URL = item.GetURL().Raw
	}
	if item.GetParent() != nil {
		s.Parent = item.GetParent().GetID()
	}
	if item.GetError() != nil {
		s.Err = item.GetError().Error()
	}
	return s
}

// Computes a SHA-256 hash of a SerializedItem
func hashItem(s SerializedItem) string {
	data, _ := json.Marshal(s)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Recursively collect all items from a seed's tree
// O(n) but required.
func flattenItem(root *models.Item) []*models.Item {
	var items []*models.Item
	var traverse func(node *models.Item)
	traverse = func(node *models.Item) {
		if node == nil {
			return
		}
		items = append(items, node)
		for _, child := range node.GetChildren() {
			traverse(child)
		}
	}
	traverse(root)
	return items
}

// Poller struct
type Poller struct {
	hub        *Hub
	interval   time.Duration
	prevHashes map[string]string

	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
}

// Creates a new Poller instance.
func NewPoller(hub *Hub, interval time.Duration) *Poller {
	return &Poller{
		hub:        hub,
		interval:   interval,
		prevHashes: make(map[string]string),
	}
}

// Begins the polling loop.
func (p *Poller) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cancel != nil {
		return // Already running
	}

	p.ctx, p.cancel = context.WithCancel(context.Background())
	go p.run()
}

// Stops the polling loop.
func (p *Poller) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
}

// main polling loop.
func (p *Poller) run() {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			if p.hub.ClientCount() == 0 {
				continue
			}
			p.poll()
		}
	}
}

// fetches reactor state, computes deltas, and sends changed items to the hub for WebSocket egress
func (p *Poller) poll() {
	// Flatten all seeds and their children.
	seeds := reactor.GetStateTableItems()
	currentItems := make(map[string]SerializedItem)
	for _, seed := range seeds {
		for _, item := range flattenItem(seed) {
			s := serializedFromItem(item)
			currentItems[s.ID] = s
		}
	}

	// Compute hashes and find deltas.
	var changed []SerializedItem
	currentHashes := make(map[string]string)
	for id, s := range currentItems {
		hash := hashItem(s)
		currentHashes[id] = hash
		if p.prevHashes[id] != hash {
			changed = append(changed, s)
		}
	}

	p.prevHashes = currentHashes

	if len(changed) > 0 {
		data, err := json.Marshal(changed)
		if err == nil {
			p.hub.Send(data)
		}
	}
}

// Singleton poller instance (lazy-initialized).
var (
	globalPoller     *Poller
	globalPollerOnce sync.Once
)

// Returns the singleton poller instance.
func GetPoller() *Poller {
	globalPollerOnce.Do(func() {
		interval := config.Get().APIFrontierPollInterval
		if interval <= 0 {
			interval = time.Second // fallback default
		}
		globalPoller = NewPoller(globalHub, interval)
	})
	return globalPoller
}
