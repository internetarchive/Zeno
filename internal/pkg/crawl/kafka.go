package crawl

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/CorentinB/Zeno/internal/pkg/frontier"
	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

type kafkaMessage struct {
	URL string `json:"url"`
}

// KafkaConnector read seeds from Kafka and ingest them into the crawl
func (crawl *Crawl) KafkaConnector() {
	// make a new reader that consumes from topic-A
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  crawl.KafkaBrokers,
		GroupID:  crawl.KafkaConsumerGroup,
		Topic:    crawl.KafkaFeedTopic,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	})

	for {
		var newKafkaMessage = new(kafkaMessage)

		m, err := r.ReadMessage(context.Background())
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error": err,
			}).Warning("Unable to read message from Kafka")
			continue
		}

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
			continue
		}

		URL, err := url.Parse(newKafkaMessage.URL)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"kafka_msg_url": newKafkaMessage.URL,
				"error":         err,
			}).Warning("Unable to parse URL from Kafka message")
			continue
		}

		newItem := frontier.NewItem(URL, nil, 0)

		crawl.Frontier.PushChan <- newItem
	}
}
