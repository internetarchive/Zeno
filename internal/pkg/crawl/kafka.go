package crawl

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/remeh/sizedwaitgroup"
	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

type kafkaMessage struct {
	URL       string `json:"u"`
	HopsCount string `json:"p"`
	ParentURL string `json:"v"`
}

func zenoHopsToHeritrixHops(hops uint8) string {
	var newHops string
	var i uint8

	for i = 0; i < hops; i++ {
		newHops = newHops + "E"
	}

	return newHops
}

// KafkaProducer receive seeds from the crawl and send them to Kafka
func (crawl *Crawl) KafkaProducer() {
	for item := range crawl.KafkaProducerChannel {
		var newKafkaMessage = new(kafkaMessage)

		w := kafka.NewWriter(kafka.WriterConfig{
			Brokers:  crawl.KafkaBrokers,
			Topic:    crawl.KafkaOutlinksTopic,
			Balancer: &kafka.LeastBytes{},
		})

		newKafkaMessage.URL = item.URL.String()
		newKafkaMessage.HopsCount = zenoHopsToHeritrixHops(item.Hop)
		if item.ParentItem != nil {
			newKafkaMessage.ParentURL = item.ParentItem.URL.String()
		}

		newKafkaMessageBytes, err := json.Marshal(newKafkaMessage)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error": err,
			}).Warning("Unable to marshal message before sending to KAfka")
		}

		err = w.WriteMessages(context.Background(),
			kafka.Message{
				Key:   nil,
				Value: newKafkaMessageBytes,
			},
		)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error": err,
			}).Warning("Failed to produce message to Kafka, pushing the seed to the local queue instead")
			crawl.Frontier.PushChan <- item
		}

		logrus.WithFields(logrus.Fields{
			"msg": string(newKafkaMessageBytes),
		}).Warning("Message sent to kafka")
	}
}

// KafkaConsumer read seeds from Kafka and ingest them into the crawl
func (crawl *Crawl) KafkaConsumer() {
	var kafkaWorkerPool = sizedwaitgroup.New(crawl.Workers)

	// make a new reader that consumes from topic-A
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  crawl.KafkaBrokers,
		GroupID:  crawl.KafkaConsumerGroup,
		Topic:    crawl.KafkaFeedTopic,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	})

	logrus.WithFields(logrus.Fields{
		"brokers": crawl.KafkaBrokers,
		"group":   crawl.KafkaConsumerGroup,
		"topic":   crawl.KafkaFeedTopic,
	}).Info("Starting Kafka consuming, it may take some time to actually start pulling messages..")

	for {
		if crawl.Finished.Get() {
			kafkaWorkerPool.Wait()
			break
		}

		if crawl.Frontier.QueueCount.Value() > int64(crawl.Workers*2) {
			time.Sleep(time.Second * 1)
			continue
		}

		kafkaWorkerPool.Add()
		go func(wg *sizedwaitgroup.SizedWaitGroup) {
			var newKafkaMessage = new(kafkaMessage)
			var newItem = new(frontier.Item)

			m, err := r.ReadMessage(context.Background())
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"error": err,
				}).Warning("Unable to read message from Kafka")
				wg.Done()
				return
			}

			logrus.WithFields(logrus.Fields{
				"value": string(m.Value),
				"key":   string(m.Key),
			}).Debug("New message received from Kafka")

			err = json.Unmarshal(m.Value, &newKafkaMessage)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"topic":     m.Topic,
					"key":       m.Key,
					"offset":    m.Offset,
					"value":     m.Value,
					"partition": m.Partition,
					"error":     err,
				}).Warning("Unable to unmarshal message from Kafka")
				wg.Done()
				return
			}

			// Parse new URL
			newURL, err := url.Parse(newKafkaMessage.URL)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"kafka_msg_url": newKafkaMessage.URL,
					"error":         err,
				}).Warning("Unable to parse URL from Kafka message")
				wg.Done()
				return
			}

			// If the message specify a parent URL, let's construct a parent item
			if len(newKafkaMessage.ParentURL) > 0 {
				newParentURL, err := url.Parse(newKafkaMessage.ParentURL)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"kafka_msg_url": newKafkaMessage.URL,
						"error":         err,
					}).Warning("Unable to parse parent URL from Kafka message")
				} else {
					newParentItem := frontier.NewItem(newParentURL, nil, 0)
					newItem = frontier.NewItem(newURL, newParentItem, 0)
				}
			} else {
				newItem = frontier.NewItem(newURL, nil, 0)
			}

			crawl.Frontier.PushChan <- newItem

			wg.Done()
		}(&kafkaWorkerPool)
	}
}
