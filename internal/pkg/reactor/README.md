# Reactor Package Documentation
## Overview
The reactor package provides functionality to manage and control the processing of seeds. It includes mechanisms for inserting seeds, receiving feedback, and marking seeds as finished. The package ensures that operations are atomic and synchronized, maintaining consistency and avoiding race conditions.  

The reactor package is designed to be used in a concurrent environment, where multiple goroutines may interact with the reactor. It uses channels and a state table to manage the flow of seeds and their processing status. The package is thread-safe and provides error handling for common scenarios.  

A token-based system is used to limit the number of seeds processed concurrently. The reactor can be initialized with a maximum number of tokens, which determines the number of seeds that can be processed simultaneously. This helps prevent overloading the system and ensures efficient resource utilization.

## Installation
To use the reactor package, import it into your package:
```go
import "github.com/internetarchive/Zeno/pkg/reactor"
```

## Usage
### Initialization
Before using the reactor, you need to initialize it with a maximum number of tokens and an output channel:
```go
outputChan := make(chan *models.Seed)
err := reactor.Start(5, outputChan)
if err != nil {
    log.Fatalf("Error starting reactor: %v", err)
}
defer reactor.Stop()
```
The initialization should happen once or it will error out with
```
ErrErrReactorAlreadyInitialized || ErrReactorNotInitialized
```

### Inserting Seeds
To insert a seed into the reactor, use the ReceiveInsert function:
```go
seed := &models.Seed{
    UUID:   uuid.New(),
    URL:    &gocrawlhq.URL{Value: "http://example.com"},
    Status: models.SeedFresh,
    Source: models.SeedSourceHQ,
}

err := reactor.ReceiveInsert(seed)
if err != nil {
    log.Fatalf("Error inserting seed: %v", err)
}
```
Inserting a seed will consume a token if available, allowing the seed to be processed. If no tokens are available, the function will block until a token is released and the seed can be inserted into the reactor.

### Feedback a Seed
To send a seed for feedback, use the ReceiveFeedback function:
```go
err := reactor.ReceiveFeedback(seed)
if err != nil {
    log.Fatalf("Error sending feedback: %v", err)
}
```
Feedback can be used to reprocess a seed that got assets added. The seed will be reinserted into the reactor without consuming a token, cause it already consumed a token when inserted.

### Marking Seeds as Finished
To mark a seed as finished, use the MarkAsFinished function:
```go
err := reactor.MarkAsFinished(seed)
if err != nil {
    log.Fatalf("Error marking seed as finished: %v", err)
}
```
Marking a seed as finished will release a token if the seed was inserted first. If the seed was not inserted, the function will error out with :
```go
ErrFinisehdItemNotFound
```

## Internals
### Reactor Struct
The reactor struct holds the state and channels for managing seed processing:
```go
type reactor struct {
	tokenPool  chan struct{}      // Token pool to control asset count
	ctx        context.Context    // Context for stopping the reactor
	cancelFunc context.CancelFunc // Context's cancel func
	input      chan *models.Seed  // Combined input channel for source and feedback
	output     chan *models.Seed  // Output channel
	stateTable sync.Map           // State table for tracking seeds by UUID
	wg         sync.WaitGroup     // WaitGroup to manage goroutines
}
```

Start Function
The Start function initializes the global reactor with the given maximum tokens and output channel. It starts the reactor's main loop in a goroutine:

Stop Function
The Stop function stops the global reactor and waits for all goroutines to finish:

Atomic Store and Send
The atomicStoreAndSend function performs a sync.Map store and a channel send atomically:

ReceiveFeedback Function
The ReceiveFeedback function sends an item to the feedback channel and ensures it is present in the state table:

ReceiveInsert Function
The ReceiveInsert function sends an item to the input channel and consumes a token:

MarkAsFinished Function
The MarkAsFinished function marks an item as finished and releases a token if found in the state table:

Run Function
The run function is the main loop of the reactor, which processes items from the input channel and sends them to the output channel:

GetStateTable Function
The GetStateTable function returns a slice of all the seeds UUIDs as strings in the state table:

Error Handling
The reactor package defines several error variables for common error scenarios:

Testing
End-to-End Test
The TestReactorE2E function provides an end-to-end test for the reactor package:

This test initializes the reactor, inserts mock seeds, processes them, and verifies that the state table is empty after processing.

Conclusion
The reactor package provides a robust and synchronized mechanism for managing seed processing. By following the usage instructions and understanding the internals, you can effectively integrate and utilize the reactor in your application.