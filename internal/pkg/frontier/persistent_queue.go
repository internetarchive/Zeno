package frontier

import (
	"path"

	"github.com/beeker1121/goque"
	"github.com/sirupsen/logrus"
)

func newPersistentQueue(jobPath string) (queue *goque.PrefixQueue, err error) {
	// Initialize a prefix queue
	queue, err = goque.OpenPrefixQueue(path.Join(jobPath, "queue"))
	if err != nil {
		logWarning.WithFields(logrus.Fields{
			"err": err.Error(),
		}).Error("Unable to create prefix queue")
		return nil, err
	}

	return queue, nil
}
