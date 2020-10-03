package frontier

import (
	"github.com/paulbellamy/ratecounter"
	"github.com/philippgille/gokv/leveldb"
)

// Seencheck holds the Seencheck database and the seen counter
type Seencheck struct {
	SeenCount *ratecounter.Counter
	SeenDB    leveldb.Store
}

// IsSeen check if the hash is in the seencheck database
func (seencheck *Seencheck) IsSeen(hash string) bool {
	var retrievedValue = new(bool)

	found, err := seencheck.SeenDB.Get(hash, &retrievedValue)
	if err != nil {
		panic(err)
	}

	if !found {
		return false
	}

	return true
}

// Seen mark a hash as seen and increment the seen counter
func (seencheck *Seencheck) Seen(hash string) {
	seencheck.SeenDB.Set(hash, true)
	seencheck.SeenCount.Incr(1)
}
