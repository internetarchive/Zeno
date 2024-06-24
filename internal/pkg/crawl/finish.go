package crawl

import (
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/frontier"
	"github.com/sirupsen/logrus"
)

// catchFinish is running in the background and detect when the crawl need to be terminated
// because it won't crawl anything more. This doesn't apply for Kafka-powered crawls.
func (crawl *Crawl) catchFinish() {
	for crawl.CrawledSeeds.Value()+crawl.CrawledAssets.Value() <= 0 {
		time.Sleep(1 * time.Second)
	}

	for {
		time.Sleep(time.Second * 5)
		if !crawl.UseHQ && crawl.ActiveWorkers.Value() == 0 && crawl.Frontier.QueueCount.Value() == 0 && !crawl.Finished.Get() && (crawl.CrawledSeeds.Value()+crawl.CrawledAssets.Value() > 0) {
			crawl.Frontier.LoggingChan <- &frontier.FrontierLogMessage{
				Fields:  logrus.Fields{},
				Message: "no more work to do, finishing",
				Level:   logrus.WarnLevel,
			}
			crawl.finish()
		}
	}
}

func (crawl *Crawl) finish() {
	crawl.WorkerStopSignal <- true
	crawl.Finished.Set(true)

	// First we wait for the queue reader to finish its current work,
	// and stop it, when it's stopped it won't dispatch any additional work
	// so we can safely close the channel it is using, and wait for all the
	// workers to notice the channel is closed, and terminate.
	crawl.Frontier.FinishingQueueReader.Set(true)
	for crawl.Frontier.IsQueueReaderActive.Get() {
		time.Sleep(time.Second / 2)
	}
	close(crawl.Frontier.PullChan)

	crawl.Logger.Warning("[WORKERS] Waiting for workers to finish")
	crawl.EnsureWorkersFinished()
	crawl.Logger.Warning("[WORKERS] All workers finished")

	// When all workers are finished, we can safely close the HQ related channels
	if crawl.UseHQ {
		crawl.Logger.Warning("[HQ] Waiting for finished channel to be closed")
		close(crawl.HQFinishedChannel)
		crawl.Logger.Warning("[HQ] Finished channel closed")

		crawl.Logger.Warning("[HQ] Waiting for producer to finish")
		close(crawl.HQProducerChannel)
		crawl.Logger.Warning("[HQ] Producer finished")

		crawl.Logger.Warning("[HQ] Waiting for all functions to return")
		crawl.HQChannelsWg.Wait()
		crawl.Logger.Warning("[HQ] All functions returned")
	}

	// Once all workers are done, it means nothing more is actively send to
	// the PushChan channel, we ask for the queue writer to terminate, and when
	// it's done we close the channel safely.
	close(crawl.Frontier.PushChan)
	crawl.Frontier.FinishingQueueWriter.Set(true)
	for crawl.Frontier.IsQueueWriterActive.Get() {
		time.Sleep(time.Second / 2)
	}

	crawl.Logger.Warning("[WARC] Closing writer(s)..")
	crawl.Client.Close()

	if crawl.Proxy != "" {
		crawl.ClientProxied.Close()
	}

	crawl.Logger.Warning("[WARC] Writer(s) closed")

	// Closing the local queue used by the frontier
	crawl.Frontier.Queue.Close()
	crawl.Logger.Warning("[FRONTIER] Queue closed")

	// Closing the seencheck database
	if crawl.Seencheck {
		crawl.Frontier.Seencheck.SeenDB.Close()
		crawl.Logger.Warning("[SEENCHECK] Database closed")
	}

	// Dumping hosts pool and frontier stats to disk
	crawl.Logger.Warning("[FRONTIER] Dumping hosts pool and frontier stats to " + path.Join(crawl.Frontier.JobPath, "frontier.gob"))
	crawl.Frontier.Save()

	crawl.Logger.Warning("Finished!")

	os.Exit(0)
}

func (crawl *Crawl) setupCloseHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	crawl.Logger.Warning("CTRL+C catched.. cleaning up and exiting.")
	signal.Stop(c)
	crawl.finish()
}
