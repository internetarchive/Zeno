package frontier

import (
	"encoding/gob"
	"os"
	"path"
	"sync"

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

	// Create a decoder
	decoder := gob.NewDecoder(decodeFile)

	// We create the structure to load the file's content
	var dump = new(sync.Map)

	// Decode the content of the file in the structure
	decoder.Decode(&dump)

	// Copy the loaded data to our actual frontier
	f.HostPool = dump

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
		logrus.Warning(err)
	}
	defer encodeFile.Close()

	// Write to the file
	var encoder = gob.NewEncoder(encodeFile)
	if err := encoder.Encode(f.HostPool); err != nil {
		logrus.Warning(err)
	}
}
