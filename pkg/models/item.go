package models

import (
	"errors"
	"fmt"
	"sync"
)

// Item represents a URL, it's children (e.g. discovered assets) and it's state in the pipeline
// The children follow a tree structure where the seed is the root and the children are the leaves, this is to keep track of the hops and the origin of the children
type Item struct {
	id         string       // ID is the unique identifier of the item
	url        *URL         // URL is a struct that contains the URL, the parsed URL, and its hop
	seed       bool         // Seed is a flag to indicate if the item is a seed or not (true=seed, false=child)
	seedVia    string       // SeedVia is the source of the seed (shoud not be used for non-seeds)
	status     ItemState    // Status is the state of the item in the pipeline
	source     ItemSource   // Source is the source of the item in the pipeline
	childrenMu sync.RWMutex // Mutex to protect the children slice
	children   []*Item      // Children is a slice of Item created from this item
	parent     *Item        // Parent is the parent of the item (will be nil if the item is a seed)
	err        error        // Error message of the seed
}

// ItemState qualifies the state of a item in the pipeline
type ItemState int64

const (
	// ItemFresh is the initial state of a item either it's from HQ, the Queue or Feedback
	ItemFresh ItemState = iota
	// ItemPreProcessed is the state after the item has been pre-processed
	ItemPreProcessed
	// ItemArchived is the state after the item has been archived
	ItemArchived
	// ItemPostProcessed is the state after the item has been post-processed
	ItemPostProcessed
	// ItemFailed is the state after the item has failed
	ItemFailed
	// ItemCompleted is the state after the item has been completed
	ItemCompleted
	// ItemGotRedirected is the state after the item has been redirected
	ItemGotRedirected
	// ItemGotChildren is the state after the item has got children
	ItemGotChildren
)

// ItemSource qualifies the source of a item in the pipeline
type ItemSource int64

const (
	// ItemSourceInsert is for items which source is not defined when inserted on reactor
	ItemSourceInsert ItemSource = iota
	// ItemSourceQueue is for items that are from the Queue
	ItemSourceQueue
	// ItemSourceHQ is for items that are from the HQ
	ItemSourceHQ
	// ItemSourcePostprocess is for items generated from redirections
	ItemSourcePostprocess
	// ItemSourceFeedback is for items that are from the Feedback
	ItemSourceFeedback
)

// CheckConsistency checks if the item is consistent with the constraints of the model
// Developers should add more constraints as needed
// Ideally this function should be called after every mutation of an item object to ensure consistency and throw a panic if consistency is broken
func (i *Item) CheckConsistency() error {
	// The item should have a URL
	if i.url == nil {
		return fmt.Errorf("url is nil")
	}

	// The item should have an ID
	if i.id == "" {
		return fmt.Errorf("id is empty")
	}

	// If item is a child, it should have a parent
	if !i.seed && i.parent == nil {
		return fmt.Errorf("item is a child but has no parent")
	}

	// If item is a seed, it shouldnt have a parent
	if i.seed && i.parent != nil {
		return fmt.Errorf("item is a seed but has a parent")
	}

	// If item is a child, it shouldnt have a seedVia
	if !i.seed && i.seedVia != "" {
		return fmt.Errorf("item is a child but has a seedVia")
	}

	return nil
}

// GetID returns the ID of the item
func (i *Item) GetID() string { return i.id }

// GetShortID returns the short ID of the item
func (i *Item) GetShortID() string { return i.id[:5] }

// GetURL returns the URL of the item
func (i *Item) GetURL() *URL { return i.url }

// IsSeed returns the seed flag of the item
func (i *Item) IsSeed() bool { return i.seed }

// IsChild returns the child flag of the item
func (i *Item) IsChild() bool { return !i.seed }

// GetSeedVia returns the seedVia of the item
func (i *Item) GetSeedVia() string { return i.seedVia }

// GetStatus returns the status of the item
func (i *Item) GetStatus() ItemState { return i.status }

// GetSource returns the source of the item
func (i *Item) GetSource() ItemSource { return i.source }

// GetMaxDepth returns the maxDepth of the item by traversing the tree
func (i *Item) GetMaxDepth() int64 {
	if len(i.GetChildren()) == 0 {
		return 0
	}
	maxDepth := int64(0)
	for _, child := range i.GetChildren() {
		childDepth := child.GetMaxDepth()
		if childDepth > maxDepth {
			maxDepth = childDepth
		}
	}
	return maxDepth + 1
}

// GetDepth returns the depth of the item
func (i *Item) GetDepth() int64 {
	if i.seed {
		return 0
	}
	return i.parent.GetDepth() + 1
}

// GetChildren returns the children of the item
func (i *Item) GetChildren() []*Item {
	i.childrenMu.RLock()
	defer i.childrenMu.RUnlock()
	return i.children
}

// GetParent returns the parent of the item
func (i *Item) GetParent() *Item { return i.parent }

// GetError returns the error of the item
func (i *Item) GetError() error { return i.err }

// GetSeed returns the seed (topmost parent) of any given item
func (i *Item) GetSeed() *Item {
	if i.seed {
		return i
	}
	for p := i.parent; p != nil; p = p.parent {
		if p.seed {
			return p
		}
	}
	return nil
}

// GetNodesAtLevel returns all the nodes at a given level in the seed
func (i *Item) GetNodesAtLevel(targetLevel int) ([]*Item, error) {
	if !i.seed {
		return nil, ErrNotASeed
	}

	var result []*Item
	var _recursiveGetNodesAtLevel func(node *Item, currentLevel int)
	_recursiveGetNodesAtLevel = func(node *Item, currentLevel int) {
		if node == nil {
			return
		}

		if currentLevel == targetLevel {
			result = append(result, node)
			return
		}

		for _, child := range node.GetChildren() {
			_recursiveGetNodesAtLevel(child, currentLevel+1)
		}
	}

	_recursiveGetNodesAtLevel(i, 0)
	return result, nil
}

// SetStatus sets the status of the item
func (i *Item) SetStatus(status ItemState) { i.status = status }

// SetSource sets the source of the item
func (i *Item) SetSource(source ItemSource) error {
	if !i.seed && (source == ItemSourceInsert || source == ItemSourceQueue || source == ItemSourceHQ) {
		return fmt.Errorf("source is invalid for a child")
	}
	i.source = source
	return nil
}

// SetError sets the error of the item
func (i *Item) SetError(err error) *Item { i.err = err; return i }

// NewItem creates a new item with the given ID, URL, seedVia and seed flag
func NewItem(ID string, URL *URL, via string, isSeed bool) *Item {
	if ID == "" || URL == nil {
		return nil
	}
	return &Item{
		id:      ID,
		url:     URL,
		seed:    isSeed,
		seedVia: via,
		status:  ItemFresh,
	}
}

// AddChild adds a child to the item
func (i *Item) AddChild(child *Item, from ItemState) error {
	i.childrenMu.Lock()
	defer i.childrenMu.Unlock()
	if from != ItemGotRedirected && from != ItemGotChildren {
		return fmt.Errorf("from state is invalid, only ItemGotRedirected and ItemGotChildren are allowed")
	}
	i.children = append(i.children, child)
	child.parent = i
	return nil
}

// IsRedirection returns true if the item is from a redirection
func (i *Item) IsRedirection() bool {
	return i.parent != nil && i.parent.status == ItemGotRedirected
}

// IsAChild returns true if the item is a child
func (i *Item) IsAChild() bool {
	return i.parent != nil && i.parent.status == ItemGotChildren
}

// HasRedirection returns true if the item has a redirection
func (i *Item) HasRedirection() bool {
	return len(i.children) == 1 && i.status == ItemGotRedirected
}

// HasChildren returns true if the item has children
func (i *Item) HasChildren() bool {
	return len(i.children) > 0 && i.status == ItemGotChildren
}

// Errors definition
var (
	// ErrNotASeed is returned when the item is not a seed
	ErrNotASeed = errors.New("item is not a seed")
)
