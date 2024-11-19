package models

import (
	"github.com/google/uuid"
	"github.com/internetarchive/gocrawlhq"
)

// Item represents a URL, it's childs (e.g. discovered assets) and it's state in the pipeline
type Item struct {
	UUID           *uuid.UUID       // UUID is the unique identifier of the item
	URL            *gocrawlhq.URL   // URL is the URL of the item
	Status         ItemState        // Status is the state of the item in the pipeline
	Source         ItemSource       // Source is the source of the item in the pipeline
	ChildsCaptured bool             // ChildsCaptured is the flag to indicate if the child URLs of the item have been captured
	Childs         []*gocrawlhq.URL // Childs is the list of URLs that have been discovered via the item's URL
}

// ItemState qualifies the state of a item in the pipeline
type ItemState int

const (
	// ItemFresh is the initial state of a item either it's from HQ, the Queue or Feedback
	ItemFresh ItemState = iota
	// ItemPreProcessed is the state after the item has been pre-processed
	ItemPreProcessed
	// ItemCaptured is the state after the item has been captured
	ItemCaptured
	// ItemPostProcessed is the state after the item has been post-processed
	ItemPostProcessed
	// ItemFailed is the state after the item has failed
	ItemFailed
	// ItemCompleted is the state after the item has been completed
	ItemCompleted
)

// ItemSource qualifies the source of a item in the pipeline
type ItemSource int

const (
	// ItemSourceInsert is for items which source is not defined when inserted on reactor
	ItemSourceInsert ItemSource = iota
	// ItemSourceQueue is for items that are from the Queue
	ItemSourceQueue
	// ItemSourceHQ is for items that are from the HQ
	ItemSourceHQ
	// ItemSourceFeedback is for items that are from the Feedback
	ItemSourceFeedback
)
