package models

import (
	"github.com/google/uuid"
	"github.com/internetarchive/gocrawlhq"
)

// Seed represents a URL, it's assets and it's state in the pipeline
type Seed struct {
	UUID           *uuid.UUID       // UUID is the unique identifier of the seed
	URL            *gocrawlhq.URL   // URL is the URL of the seed
	Status         SeedState        // Status is the state of the seed in the pipeline
	Source         SeedSource       // Source is the source of the seed in the pipeline
	AssetsCaptured bool             // AssetsCaptured is the flag to indicate if the assets of the seed has been captured
	Assets         []*gocrawlhq.URL // Assets is the list of assets of the seed
}

// SeedState qualifies the state of a seed in the pipeline
type SeedState int

const (
	// SeedFresh is the initial state of a seed either it's from HQ, the Queue or Feedback
	SeedFresh SeedState = iota
	// SeedPreProcessed is the state after the seed has been pre-processed
	SeedPreProcessed
	// SeedCaptured is the state after the seed has been captured
	SeedCaptured
	// SeedPostProcessed is the state after the seed has been post-processed
	SeedPostProcessed
	// SeedFailed is the state after the seed has failed
	SeedFailed
	// SeedCompleted is the state after the seed has been completed
	SeedCompleted
)

// SeedSource qualifies the source of a seed in the pipeline
type SeedSource int

const (
	// SeedSourceQueue is for seeds that are from the Queue
	SeedSourceQueue SeedSource = iota
	// SeedSourceHQ is for seeds that are from the HQ
	SeedSourceHQ
	// SeedSourceFeedback is for seeds that are from the Feedback
	SeedSourceFeedback
)
