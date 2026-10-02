package hq4

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/internetarchive/Zeno/v2/internal/pkg/log"
	"github.com/internetarchive/Zeno/v2/internal/pkg/reactor"
	"github.com/internetarchive/Zeno/v2/internal/pkg/source"
	"github.com/internetarchive/Zeno/v2/pkg/models"
	"github.com/internetarchive/gocrawlhq"
	amqp "github.com/rabbitmq/amqp091-go"
)

var errMissingID = errors.New("message has no ID")

// consumer reads the deliveries from RabbitMQ and sends them to the reactor.
func (s *HQ) consumer() {
	defer s.wg.Done()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "rabbitmq.consumer",
	})

	r := source.NewFeedEmptyReporter(logger)
	idleTicker := time.NewTicker(500 * time.Millisecond)
	defer idleTicker.Stop()

	deliveries := s.rabbit.Deliveries()

	for {
		select {
		case <-s.ctx.Done():
			logger.Debug("channel closed")
			return
		case <-idleTicker.C:
			r.Report(0)
		case delivery, ok := <-deliveries:
			if !ok {
				// the rabbit source already requested the shutdown and the channel is closed or stopped.
				<-s.ctx.Done()
				logger.Debug("closed after RabbitMQ disconnected")
				return
			}
			r.Report(1)
			idleTicker.Reset(500 * time.Millisecond)

			if done := s.consumeDelivery(logger, delivery); done {
				<-s.ctx.Done()
				logger.Debug("channel ran into an error while sending to reactor")
				return
			}
		}
	}
}

// consumeDelivery sends the delivery's URL to the reactor
func (s *HQ) consumeDelivery(logger *log.FieldedLogger, delivery amqp.Delivery) (frozen bool) {
	URL, err := decodeDelivery(delivery)
	if err != nil {
		logger.Error("discarding invalid message", "err", err, "body", string(delivery.Body))
		ackOrLog(logger, delivery.Reject(false), URL.ID)
		return false
	}

	// ID _could_ be duplicated without seencheck. track will return false if a duplicate is spotted.
	if !s.track(URL.ID, delivery) {
		logger.Debug("discarding duplicate seed", "id", URL.ID, "url", URL.Value)
		ackOrLog(logger, delivery.Ack(false), URL.ID)
		return false
	}

	parsedURL, err := models.NewURL(URL.Value)
	if err != nil {
		logger.Debug("URL parsing failed. untracking and failing URL", "url", URL.Value, "err", err)
		s.untrack(URL.ID)
		ackOrLog(logger, delivery.Ack(false), URL.ID)
		return false
	}

	parsedURL.SetHops(pathToHops(URL.Path))

	// Create new ID from the delivery struct
	newItem := models.NewItemWithID(URL.ID, &parsedURL, URL.Via)
	newItem.SetSource(models.ItemSourceHQ)

	logger.Debug("sending newly created item to reactor", "item", newItem.GetShortID())

	err = reactor.ReceiveInsert(newItem)
	if err != nil {
		// errors here are fatal and likely should be left unacked.
		s.untrack(URL.ID)
		if err == reactor.ErrReactorFrozen {
			return true
		}

		// errors are incredibly unlikely and should be investigated.
		panic(err)
	}

	return false
}

func decodeDelivery(delivery amqp.Delivery) (URL gocrawlhq.URL, err error) {
	if err = json.Unmarshal(delivery.Body, &URL); err != nil {
		return URL, err
	}
	if URL.ID == "" {
		return URL, errMissingID
	}
	return URL, nil
}

// track records the delivery struct into s.pending to ack once the seed is crawled
func (s *HQ) track(ID string, delivery amqp.Delivery) bool {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	if _, inFlight := s.pending[ID]; inFlight {
		return false
	}
	s.pending[ID] = delivery
	return true
}

// untrack removes ID from map and returns the delivery struct
func (s *HQ) untrack(ID string) (amqp.Delivery, bool) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	delivery, ok := s.pending[ID]
	delete(s.pending, ID)
	return delivery, ok
}

func (s *HQ) pendingCount() int {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	return len(s.pending)
}

func ackOrLog(logger *log.FieldedLogger, err error, ID string) {
	if err != nil {
		// the channel is not accessible to ack the message. this will be requeued by rabbit. :/
		logger.Error("unable to settle RabbitMQ delivery", "id", ID, "err", err)
	}
}
