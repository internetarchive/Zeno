package crawl

import (
	"encoding/json"
	"net/url"
	"time"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/remeh/sizedwaitgroup"
	"github.com/sirupsen/logrus"
)

type kafkaMessage struct {
	URL       string `json:"u"`
	HopsCount uint8  `json:"hop"`
	ParentURL string `json:"parent_url"`
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
	p, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": crawl.KafkaBrokers})
	if err != nil {
		panic(err)
	}
	defer p.Close()

	// Delivery report handler for produced messages
	go func() {
		for e := range p.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					log.WithFields(logrus.Fields{
						"error":     ev.TopicPartition.Error,
						"partition": ev.TopicPartition,
						"msg":       ev.String(),
					}).Warning("Kafka message delivery failed")
				} else {
					log.WithFields(logrus.Fields{
						"partition": ev.TopicPartition,
						"msg":       ev.String(),
					}).Debug("Kafka message delivered")
				}
			}
		}
	}()

	for item := range crawl.KafkaProducerChannel {
		if crawl.Finished.Get() {
			break
		}

		var newKafkaMessage = new(kafkaMessage)

		newKafkaMessage.URL = item.URL.String()
		newKafkaMessage.HopsCount = item.Hop
		if item.ParentItem != nil {
			newKafkaMessage.ParentURL = item.ParentItem.URL.String()
		}

		newKafkaMessageBytes, err := json.Marshal(newKafkaMessage)
		if err != nil {
			log.WithFields(logrus.Fields{
				"error": err,
			}).Warning("Unable to marshal message before sending to KAfka")
		}

		err = p.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &crawl.KafkaOutlinksTopic, Partition: kafka.PartitionAny},
			Value:          newKafkaMessageBytes,
		}, nil)
		if err != nil {
			log.WithFields(logrus.Fields{
				"error": err,
			}).Warning("Failed to produce message to Kafka, pushing the seed to the local queue instead")
			crawl.Frontier.PushChan <- item
		}
	}

	// Wait for message deliveries before shutting down
	p.Flush(15 * 1000)
}

// KafkaConsumer read seeds from Kafka and ingest them into the crawl
func (crawl *Crawl) KafkaConsumer() {
	var kafkaWorkerPool = sizedwaitgroup.New(16)

	kafkaClient, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": crawl.KafkaBrokers,
		"group.id":          crawl.KafkaConsumerGroup,
	})
	if err != nil {
		panic(err)
	}

	kafkaClient.SubscribeTopics([]string{crawl.KafkaFeedTopic}, nil)

	logrus.WithFields(logrus.Fields{
		"brokers": crawl.KafkaBrokers,
		"group":   crawl.KafkaConsumerGroup,
		"topic":   crawl.KafkaFeedTopic,
	}).Info("Kafka consumer started, it may take some time to actually start pulling messages..")

	for {
		if crawl.Finished.Get() {
			kafkaWorkerPool.Wait()
			kafkaClient.Close()
			break
		}

		if crawl.ActiveWorkers.Value() >= int64(crawl.Workers-(crawl.Workers/10)) {
			time.Sleep(time.Second * 1)
			continue
		}

		kafkaWorkerPool.Add()
		go func(wg *sizedwaitgroup.SizedWaitGroup) {
			var newKafkaMessage = new(kafkaMessage)
			var newItem = new(frontier.Item)
			var newParentItemHops uint8

			msg, err := kafkaClient.ReadMessage(15)
			if err != nil {
				log.WithFields(logrus.Fields{
					"error": err,
				}).Warning("Unable to read message from Kafka")
				wg.Done()
				return
			}

			log.WithFields(logrus.Fields{
				"value": string(msg.Value),
				"key":   string(msg.Key),
			}).Debug("New message received from Kafka")

			err = json.Unmarshal(msg.Value, &newKafkaMessage)
			if err != nil {
				log.WithFields(logrus.Fields{
					"topic":     crawl.KafkaFeedTopic,
					"key":       msg.Key,
					"value":     msg.Value,
					"partition": msg.TopicPartition,
					"error":     err,
				}).Warning("Unable to unmarshal message from Kafka")
				wg.Done()
				return
			}

			// Parse new URL
			newURL, err := url.Parse(newKafkaMessage.URL)
			if err != nil {
				log.WithFields(logrus.Fields{
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
					log.WithFields(logrus.Fields{
						"kafka_msg_url": newKafkaMessage.URL,
						"error":         err,
					}).Warning("Unable to parse parent URL from Kafka message")
				} else {
					if newKafkaMessage.HopsCount > 0 {
						newParentItemHops = newKafkaMessage.HopsCount - 1
					}
					newParentItem := frontier.NewItem(newParentURL, nil, newParentItemHops)
					newItem = frontier.NewItem(newURL, newParentItem, newKafkaMessage.HopsCount)
				}
			} else {
				newItem = frontier.NewItem(newURL, nil, newKafkaMessage.HopsCount)
			}

			crawl.Frontier.PushChan <- newItem
			wg.Done()
		}(&kafkaWorkerPool)
	}
}
