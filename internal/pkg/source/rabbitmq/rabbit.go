package hqr

import (
	"errors"
	"log/slog"
	"strconv"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	reconnectDelay = 5 * time.Second
	reInitDelay    = 2 * time.Second
	resendDelay    = 5 * time.Second
	maxRetryLimit  = 10
)

type SourceRabbit struct {
	connection *amqp.Connection
	// async channel for initializing project-associated channels
	RCIncoming      chan RabbitChannel
	rc              *RabbitChannel
	done            chan bool
	notifyConnClose chan *amqp.Error
	// TODO: write tests to validate connection failure states
	connectionReady *atomic.Bool
}

// Client struct for managing rabbit channels
// TODO: Does this need to be thread-safe?
type RabbitChannel struct {
	projectUUID       string
	queue             RabbitQueue
	channel           *amqp.Channel
	notifyChanClose   chan *amqp.Error
	notifyConfirm     chan amqp.Confirmation
	channelReady      *atomic.Bool
	notifyInitSuccess chan error
}

// Client struct for managing queue metadata
// TODO: Does this need to be thread-safe?
type RabbitQueue struct {
	queueReady   *atomic.Bool
	queueName    string
	exchangeName string
	routingKey   string
}

// New creates a new consumer state instance, and automatically
// attempts to connect to the server.
func NewSourceRabbit(addr, projectUUID, routingKey string) *SourceRabbit {
	rc := NewRabbitChannel(projectUUID, routingKey)
	src := SourceRabbit{
		RCIncoming:      make(chan RabbitChannel),
		rc:              &rc,
		done:            make(chan bool),
		connectionReady: &atomic.Bool{},
	}
	go src.handleReconnect(addr)
	return &src
}

func NewRabbitChannel(projectUUID, routingKey string) RabbitChannel {
	return RabbitChannel{
		projectUUID: projectUUID,
		queue: RabbitQueue{
			queueReady:   &atomic.Bool{},
			queueName:    projectUUID,
			exchangeName: projectUUID,
			routingKey:   routingKey,
		},
		channelReady:      &atomic.Bool{},
		notifyInitSuccess: make(chan error),
	}
}

func (src *SourceRabbit) handleReconnect(addr string) {
	for {
		src.connectionReady.Store(false)
		// establish a connection
		_, err := src.connect(addr)

		if err != nil {
			slog.Error("unable to connect to rabbitmq server", "err", err.Error())

			select {
			case <-src.done:
				return
			case <-time.After(reconnectDelay):
			}
			continue
		}

		if done := src.handleProjectChannelInit(); done {
			break
		}

	}
}

// connect will create a new AMQP connection
func (src *SourceRabbit) connect(addr string) (*amqp.Connection, error) {
	conn, err := amqp.Dial(addr)
	if err != nil {
		return nil, err
	}

	src.changeConnection(conn)
	slog.Info("RabbitMQ server connection successful!")
	return conn, nil
}

// changeConnection takes a new connection to the queue,
// and updates the close listener to reflect this.
func (src *SourceRabbit) changeConnection(connection *amqp.Connection) {
	src.connection = connection
	src.notifyConnClose = make(chan *amqp.Error, 1)
	src.connection.NotifyClose(src.notifyConnClose)
	src.connectionReady.Store(true)
}

func (src *SourceRabbit) handleProjectChannelInit() (done bool) {
	// receieve from ChanChan and create a channel
	// loop thru RabbitQueue slice and configure queue + exchange
	for {
		if !src.connectionReady.Load() {
			slog.Error("need connection to initialize channels")
		}

		src.handleProjectReconnect()

		select {
		case <-src.done:
			return true
		case <-src.notifyConnClose:
			slog.Error("Connection closed. Reconnecting...")
			return false
		case <-src.rc.notifyChanClose:
			slog.Error("Channel closed. Reconnecting...")
		}
	}
}

func (src *SourceRabbit) handleProjectReconnect() {
	src.rc.channelReady.Store(false)
	numberOfAttempts := 0
	for {
		err := src.initProject()
		if err != nil {
			numberOfAttempts++
			if numberOfAttempts >= maxRetryLimit {
				src.rc.notifyInitSuccess <- errors.New("failed to initialize project channel, exceeded maxRetryLimit")
			}
			slog.Error("error initializing project channel. Trying again in "+strconv.FormatFloat(reInitDelay.Seconds(), 'f', -1, 64)+" seconds", "err", err.Error())
			<-time.After(reInitDelay)
		} else {
			// channel created successfully
			break
		}
	}
}

func (src *SourceRabbit) initProject() (err error) {
	src.rc.channel, err = src.handleChannel()

	if err != nil {
		slog.Error("error creating channel", "err", err)
		return err
	}

	src.rc.queue.queueReady.Store(false)
	err = src.configureProjectExchange(src.rc.channel, src.rc.queue)
	if err != nil {
		slog.Error("error configuring exchange", "err", err)
		return err
	}
	err = src.configureQueue(src.rc.channel, src.rc.queue)
	if err != nil {
		slog.Error("error configuring queue", "err", err)
		return err
	}

	src.rc.queue.queueReady.Store(true)

	src.rc.changeChannel(src.rc.channel)
	src.rc.notifyInitSuccess <- nil

	return nil
}

func (rc *RabbitChannel) changeChannel(channel *amqp.Channel) {
	rc.channel = channel
	rc.notifyChanClose = make(chan *amqp.Error, 1)
	rc.channel.NotifyClose(rc.notifyChanClose)
	rc.channelReady.Store(true)
}

func (src *SourceRabbit) handleChannel() (ch *amqp.Channel, err error) {
	ch, err = src.connection.Channel()
	// TODO: defer ch.Close() in main logic loop

	if err != nil {
		return
	}

	err = ch.Qos(
		1000,
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
			// "x-delivery-limit": 20,
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
	if err != nil {
		return
	}

	q.queueReady.Store(true)

	return
}
