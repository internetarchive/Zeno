package frontier

import (
	"encoding/gob"
	"os"
	"path"
	"time"

	"github.com/paulbellamy/ratecounter"
	"github.com/sirupsen/logrus"
)

type frontierStats struct {
	Hosts       map[string]*ratecounter.Counter
	QueuedCount int64
}

// Load take the path to the frontier's hosts pool and status dump
// it decodes that file and load it in the job's frontier
func (f *Frontier) Load() {
	// Open a RO file
	decodeFile, err := os.OpenFile(path.Join(f.JobPath, "frontier.gob"), os.O_RDONLY, 0644)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err,
		}).Warning("Unable to load Frontier stats and host pool, it is not a problem if you are starting this job for the first time")
		return
	}
	defer decodeFile.Close()

	// Create a decoder
	decoder := gob.NewDecoder(decodeFile)

	// We create the structure to load the file's content
	var dump = new(frontierStats)
	dump.QueuedCount = f.QueueCount.Value()
	dump.Hosts = make(map[string]*ratecounter.Counter, 0)

	// Decode the content of the file in the structure
	decoder.Decode(&dump)

	// Copy the loaded data to our actual frontier
	f.HostPool.Hosts = dump.Hosts
	f.QueueCount.Incr(dump.QueuedCount)

	logrus.WithFields(logrus.Fields{
		"queued": f.QueueCount.Value(),
		"hosts":  len(f.HostPool.Hosts),
	}).Info("Successfully loaded previous frontier's hosts pool and queued URLs count")
}

// Save write the in-memory hosts pool and queued count to file
// to resume properly the next time the job is loaded
func (f *Frontier) Save() {
	// Create a file for IO
	encodeFile, err := os.OpenFile(path.Join(f.JobPath, "frontier.gob"), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logrus.Warning(err)
	}
	defer encodeFile.Close()

	// We create the structure to save to the file,
	// it's a copy of the hosts pool and the count
	// of the queued items
	var dump = new(frontierStats)
	dump.QueuedCount = f.QueueCount.Value()
	dump.Hosts = make(map[string]*ratecounter.Counter, 0)

	// Recreate a clean host pool by not copying the hosts
	// that do not have any entry in queue
	f.HostPool.Mutex.Lock()
	var hosts = f.HostPool.Hosts
	f.HostPool.Mutex.Unlock()

	for host, hostCounter := range hosts {
		if hostCounter.Value() != 0 {
			dump.Hosts[host] = hostCounter
		}
	}

	// Write to the file
	var encoder = gob.NewEncoder(encodeFile)
	if err := encoder.Encode(dump); err != nil {
		logrus.Warning(err)
	}
}

func (f *Frontier) writeFrontierToDisk() {
	for {
		f.Save()
		time.Sleep(time.Minute * 1)
	}
}
