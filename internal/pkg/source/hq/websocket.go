package hq

import (
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/utils"
	"github.com/internetarchive/gocrawlhq"
)

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
