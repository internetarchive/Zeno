package hq

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/gobwas/ws/wsutil"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/internetarchive/gocrawlhq"
)

var unknownMsgTypeErr = errors.New("unknown HQ websocket message type")

// websocket connects to HQ's websocket and listen for messages.
// It also sends and "identify" message to the HQ to let it know that
// Zeno is connected. This "identify" message is sent every second and
// contains the crawler's stats and details.
func (s *HQ) websocket() {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "hq.websocket",
	})

	identifyTicker := time.NewTicker(time.Second)
	defer identifyTicker.Stop()

	go s.listenMessages()

	for {
		select {
		case <-s.ctx.Done():
			logger.Debug("received done signal")
			s.wg.Done()
			return
		default:
			s.sendIdentify(logger)
			<-identifyTicker.C
		}
	}
}

const readTimeout = 30 * time.Second

func (s *HQ) listenMessages() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			conn := *s.client.WebsocketConn
			conn.SetReadDeadline(time.Now().Add(readTimeout))
			msgs, err := wsutil.ReadServerMessage(conn, nil)
			conn.SetReadDeadline(time.Time{}) // Reset read deadline
			if err != nil {
				logger.Error("error reading message from HQ websocket, retrying", "err", err)
				time.Sleep(5 * time.Second)
				continue
			}
			for _, msg := range msgs {
				mType, err := dispatchMessageByType(bytes.TrimSpace(msg.Payload))
				if err != nil {
					logger.Error("error dispatching message by type", "msg_type", mType, "err", err)
					continue
				}
			}
		}
	}
}

func dispatchMessageByType(msg []byte) (string, error) {
	type msgType struct {
		Type string `json:"type"`
	}

	var m msgType
	var err error

	if err = json.Unmarshal(msg, &m); err != nil {
		return "", err
	}

	switch m.Type {
	case "signal":
		err = handleSignalMsg(msg)
	case "confirmed":
		err = handleConfirmedMsg(msg)
	default:
		err = unknownMsgTypeErr
	}
	return m.Type, err
}

func handleSignalMsg(msg []byte) error {
	type signalMsg struct {
		Signal int `json:"signal"`
	}
	var m signalMsg

	if err := json.Unmarshal(msg, &m); err != nil {
		return err
	}

	logger.Warn("sending signal to process", "signal", m.Signal, "pid", os.Getpid(), "payload", string(msg))

	p, err1 := os.FindProcess(os.Getpid())
	err2 := p.Signal(syscall.Signal(m.Signal))
	if err1 != nil || err2 != nil {
		return fmt.Errorf("error sending signal %d to process %d: %v, %v", m.Signal, os.Getpid(), err1, err2)
	}

	return nil
}

func handleConfirmedMsg(msg []byte) error {
	type confirmedMsg struct {
		Type    string `json:"type"`
		Payload struct {
			Project    string `json:"project"`
			Job        string `json:"job"`
			IP         string `json:"ip"`
			Hostname   string `json:"hostname"`
			Identifier string `json:"identifier"`
			Timestamp  int64  `json:"timestamp"`
			GoVersion  string `json:"goVersion"`
		} `json:"payload"`
	}
	var m confirmedMsg

	if err := json.Unmarshal(msg, &m); err != nil {
		return err
	}

	return nil
}

func (s *HQ) sendIdentify(logger *log.FieldedLogger) {
	err := s.client.Identify(&gocrawlhq.IdentifyMessage{
		Project:   config.Get().HQProject,
		Job:       config.Get().Job,
		IP:        utils.GetOutboundIP().String(),
		Hostname:  utils.GetHostname(),
		GoVersion: utils.GetVersion().GoVersion,
	})
	if err != nil {
		logger.Error("error sending identify payload to Crawl HQ, trying to reconnect", "err", err.Error())
		s.reconnectWebsocket(logger)
	}
}

func (s *HQ) reconnectWebsocket(logger *log.FieldedLogger) {
	err := s.client.InitWebsocketConn()
	if err != nil {
		logger.Error("error initializing websocket connection to Crawl HQ", "err", err.Error())
	}
}
