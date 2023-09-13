package frontier

import (
	"path"

	"github.com/beeker1121/goque"
	"github.com/sirupsen/logrus"
)

func (f *Frontier) newPersistentQueue(jobPath string) (queue *goque.PrefixQueue, err error) {
	// Initialize a prefix queue
	queue, err = goque.OpenPrefixQueue(path.Join(jobPath, "queue"))
	if err != nil {
		f.LoggingChan <- &FrontierLogMessage{
			Fields: logrus.Fields{
				"err": err.Error(),
			},
			Message: "unable to open prefix queue",
			Level:   logrus.ErrorLevel,
		}

		return nil, err
	}

	return queue, nil
}
