package frontier

import (
	"os"
	"path"

	"github.com/sirupsen/logrus"
)

// Load take the path to the frontier's hosts pool and status dump
// it decodes that file and load it in the job's frontier
func (f *Frontier) Load() {
	// Open a RO file
	decodeFile, err := os.OpenFile(path.Join(f.JobPath, "frontier.gob"), os.O_RDONLY, 0644)
	if err != nil {
		f.LoggingChan <- &FrontierLogMessage{
			Fields: logrus.Fields{
				"err": err.Error(),
			},
			Message: "unable to load Frontier stats and host pool, it is not a problem if you are starting this job for the first time",
			Level:   logrus.WarnLevel,
		}

		return
	}
	defer decodeFile.Close()

	if err := SyncMapDecode(f.HostPool, decodeFile); err != nil {
		f.LoggingChan <- &FrontierLogMessage{
			Fields: logrus.Fields{
				"err": err.Error(),
			},
			Message: "unable to decode Frontier stats and host pool",
			Level:   logrus.WarnLevel,
		}
	}

	f.LoggingChan <- &FrontierLogMessage{
		Fields: logrus.Fields{
			"hosts": f.GetHostsCount(),
		},
		Message: "successfully loaded previous frontier's hosts pool",
		Level:   logrus.InfoLevel,
	}
}

// Save write the in-memory hosts pool to resume properly the next time the job is loaded
func (f *Frontier) Save() {
	// Create a file for IO
	encodeFile, err := os.OpenFile(path.Join(f.JobPath, "frontier.gob"), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		f.LoggingChan <- &FrontierLogMessage{
			Fields: logrus.Fields{
				"err": err.Error(),
			},
			Message: "unable to open Frontier file",
			Level:   logrus.WarnLevel,
		}
	}
	defer encodeFile.Close()

	// Write to the file

	if err := SyncMapEncode(f.HostPool, encodeFile); err != nil {
		f.LoggingChan <- &FrontierLogMessage{
			Fields: logrus.Fields{
				"err": err.Error(),
			},
			Message: "unable to save Frontier stats and host pool",
			Level:   logrus.WarnLevel,
		}
	}
}
