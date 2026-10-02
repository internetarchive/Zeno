package hq4

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/internetarchive/Zeno/v2/internal/pkg/log"
	"github.com/internetarchive/Zeno/v2/internal/pkg/utils"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	// decides how long startup and shutdown can take if the broker is unreachable
	dialTimeout  = 10 * time.Second
	closeTimeout = 5 * time.Second

	// routing keys that HQ publishes with
	routingKeySeed     = "seed"
	routingKeyOutlinks = "outlinks"
)

// SourceRabbit consumes a queue that hq publishes to.
type SourceRabbit struct {
	addr        string
	prefetch    int
	consumerTag string
	rc          *RabbitChannel
	// called once the consumer is lost which stops Zeno safely
	onDisconnect func()

	deliveries      chan amqp.Delivery
	connection      *amqp.Connection
	notifyConnClose chan *amqp.Error
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
}

// Client struct for managing rabbit channels
type RabbitChannel struct {
	projectUUID string
	queues      []RabbitQueue
	// specifies which queue Zeno will consume from
	consumeQueue    RabbitQueue
	channel         *amqp.Channel
	notifyChanClose chan *amqp.Error
}

// Client struct for managing queue metadata
type RabbitQueue struct {
	queueName    string
	exchangeName string
	routingKey   string
}

func NewSourceRabbit(addr, projectUUID, routingKey string, prefetch int, onDisconnect func()) (*SourceRabbit, error) {
	rc, err := NewRabbitChannel(projectUUID, routingKey)
	if err != nil {
		return nil, err
	}

	if prefetch <= 0 {
		return nil, fmt.Errorf("rabbitmq prefetch must be positive, got %d", prefetch)
	}

	return &SourceRabbit{
		addr:         addr,
		prefetch:     prefetch,
		consumerTag:  "zeno-" + utils.GetHostname(),
		rc:           rc,
		onDisconnect: onDisconnect,
		deliveries:   make(chan amqp.Delivery),
	}, nil
}

// Mirrors how HQ declares a project in rabbitmq
func NewRabbitChannel(projectUUID, routingKey string) (*RabbitChannel, error) {
	rc := &RabbitChannel{
		projectUUID: projectUUID,
		queues: []RabbitQueue{
			{
				queueName:    projectUUID,
				exchangeName: projectUUID,
				routingKey:   routingKeySeed,
			},
			{
				queueName:    projectUUID + "-outlinks",
				exchangeName: projectUUID,
				routingKey:   routingKeyOutlinks,
			},
		},
	}

	for _, q := range rc.queues {
		if q.routingKey == routingKey {
			rc.consumeQueue = q
			return rc, nil
		}
	}

	return nil, fmt.Errorf("unknown routing key %q, must be %q or %q", routingKey, routingKeySeed, routingKeyOutlinks)
}

// Starts the connection to rabbitmq and consuming the project.
func (src *SourceRabbit) Start() error {
	msgs, err := src.setup()
	if err != nil {
		src.close()
		return err
	}

	src.ctx, src.cancel = context.WithCancel(context.Background())
	src.wg.Add(1)
	go src.run(msgs)

	return nil
}

func (src *SourceRabbit) Stop() {
	if src.cancel == nil {
		return
	}
	src.cancel()
	src.wg.Wait()
}

func (src *SourceRabbit) Deliveries() <-chan amqp.Delivery {
	return src.deliveries
}

func (src *SourceRabbit) run(msgs <-chan amqp.Delivery) {
	defer src.wg.Done()
	defer close(src.deliveries)
	defer src.close()

	logger := log.NewFieldedLogger(&log.Fields{
		"component": "hq4.rabbit",
	})

	logger.Info("consuming from rabbitmq", "queue", src.rc.consumeQueue.queueName, "prefetch", src.prefetch)

	for {
		select {
		case <-src.ctx.Done():
			return
		case msg, ok := <-msgs:
			if !ok {
				// if the channel is closed, drops, or otherwise breaks, we should need to shutdown.
				logger.Error("rabbitmq stopped delivering, shutting Zeno down", "queue", src.rc.consumeQueue.queueName, "err", src.closeReason())
				src.onDisconnect()
				return
			}

			select {
			case <-src.ctx.Done():
				// if context is closed, this will be requeued by rabbit.
				return
			case src.deliveries <- msg:
			}
		}
	}
}

// closeReason gathers the reason why rabbit closed.
func (src *SourceRabbit) closeReason() error {
	for _, notify := range []chan *amqp.Error{src.notifyConnClose, src.rc.notifyChanClose} {
		select {
		case amqpErr, ok := <-notify:
			if ok && amqpErr != nil {
				return amqpErr
			}
		default:
		}
	}
	return errors.New("consumer cancelled by the broker")
}

// setup opens the connection and channel and starts consuming.
func (src *SourceRabbit) setup() (<-chan amqp.Delivery, error) {
	conn, err := amqp.DialConfig(src.addr, amqp.Config{
		Dial: amqp.DefaultDial(dialTimeout),
	})
	if err != nil {
		return nil, fmt.Errorf("error connecting to rabbit: %w", err)
	}

	src.connection = conn
	src.notifyConnClose = conn.NotifyClose(make(chan *amqp.Error, 1))

	ch, err := src.handleChannel()
	if err != nil {
		return nil, fmt.Errorf("error creating channel: %w", err)
	}
	src.rc.channel = ch
	src.rc.notifyChanClose = ch.NotifyClose(make(chan *amqp.Error, 1))

	for _, queue := range src.rc.queues {
		if err := src.configureProjectExchange(ch, queue); err != nil {
			return nil, fmt.Errorf("error configuring exchange %s: %w", queue.exchangeName, err)
		}
		if err := src.configureQueue(ch, queue); err != nil {
			return nil, fmt.Errorf("error configuring queue %s: %w", queue.queueName, err)
		}
	}

	msgs, err := ch.Consume(
		src.rc.consumeQueue.queueName,
		src.consumerTag,
		false, // auto-ack: acked by the finisher once the seed is crawled
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("error consuming queue %s: %w", src.rc.consumeQueue.queueName, err)
	}

	return msgs, nil
}

func (src *SourceRabbit) close() {
	if src.connection != nil && !src.connection.IsClosed() {
		// closing the connection closes its channel as well
		src.connection.CloseDeadline(time.Now().Add(closeTimeout))
	}
}

func (src *SourceRabbit) handleChannel() (ch *amqp.Channel, err error) {
	ch, err = src.connection.Channel()
	if err != nil {
		return
	}

	// Caps the unacked deliveries, i.e. seeds being crawled plus those waiting for the reactor
	err = ch.Qos(
		src.prefetch,
		0,
		false,
	)
	if err != nil {
		return
	}

	return
}

func (src *SourceRabbit) configureProjectExchange(ch *amqp.Channel, q RabbitQueue) (err error) {
	err = ch.ExchangeDeclare(
		q.exchangeName,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)

	return
}

func (src *SourceRabbit) configureQueue(ch *amqp.Channel, q RabbitQueue) (err error) {
	_, err = ch.QueueDeclare(
		q.queueName,
		true,
		false,
		false,
		false,
		amqp.Table{
			amqp.QueueTypeArg: amqp.QueueTypeQuorum,
		},
	)
	if err != nil {
		return
	}

	err = ch.QueueBind(
		q.queueName,
		q.routingKey,
		q.exchangeName,
		false,
		nil,
	)

	return
}
