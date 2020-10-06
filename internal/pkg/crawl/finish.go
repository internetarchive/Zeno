package crawl

import (
	"os"
	"path"
	"time"

	"github.com/sirupsen/logrus"
)

// catchFinish is running in the background and detect when the crawl need to be terminated
// because it won't crawl anything more. This doesn't apply for Kafka-powered crawls.
func (c *Crawl) catchFinish() {
	if len(c.KafkaFeedTopic) > 0 {
		return
	}

	go func() {
		for c.Crawled.Value() <= 0 {
			time.Sleep(1 * time.Second)
		}

		for {
			time.Sleep(time.Second * 5)
			if c.ActiveWorkers.Value() == 0 && c.Frontier.QueueCount.Value() == 0 && c.Finished.Get() == false && c.Crawled.Value() > 0 {
				logrus.Warning("No additional URL to archive, finishing")
				c.Finish()
				os.Exit(0)
			}
		}
	}()
}

// Finish handle the closing of the different crawl components
func (c *Crawl) Finish() {
	c.Finished.Set(true)

	c.WorkerPool.Wait()
	logrus.Warning("All workers finished")

	if c.WARC {
		close(c.WARCWriter)
		<-c.WARCWriterFinish
		logrus.Warning("WARC writer closed")
	}

	c.Frontier.Queue.Close()
	logrus.Warning("Frontier queue closed")

	if c.Seencheck {
		c.Frontier.Seencheck.SeenDB.Close()
		logrus.Warning("Seencheck database closed")
	}

	logrus.Warning("Dumping hosts pool and frontier stats to " + path.Join(c.Frontier.JobPath, "frontier.gob"))
	c.Frontier.Save()

	logrus.Warning("Finished")
}
