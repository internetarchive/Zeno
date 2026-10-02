package hq4

import (
	"github.com/internetarchive/Zeno/v2/internal/pkg/log"
	"github.com/internetarchive/Zeno/v2/pkg/models"
)

// finisher acks the RabbitMQ delivery of seeds we have crawled.
func (s *HQ) finisher() {
	defer s.wg.Done()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "rabbitmq.finisher",
	})

	for {
		select {
		case item := <-s.finishCh:
			s.ackFinished(logger, item)
		case <-s.ctx.Done():
			// Zeno is closing; we should try to ack anything left to prevent recrawl.
			for {
				select {
				case item := <-s.finishCh:
					s.ackFinished(logger, item)
				default:
					logger.Debug("finished channel is empty. nothing left to ack for rabbit.")
					return
				}
			}

		}
	}
}

func (s *HQ) ackFinished(logger *log.FieldedLogger, item *models.Item) {
	logger.Debug("rabbit finisher received item", "item", item.GetShortID())

	delivery, ok := s.untrack(item.GetID())
	if !ok {
		logger.Error("finished item has no rabbit delivery in map to ack?", "item", item.GetShortID())
		return
	}

	// use delivery struct we got to ack message to rabbit
	ackOrLog(logger, delivery.Ack(false), item.GetID())
}
