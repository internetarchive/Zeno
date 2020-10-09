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
func (seencheck *Seencheck) IsSeen(hash string) (found bool, value string) {
	var retrievedValue string

	found, err := seencheck.SeenDB.Get(hash, &retrievedValue)
	if err != nil {
		panic(err)
	}

	return found, value
}

// Seen mark a hash as seen and increment the seen counter
func (seencheck *Seencheck) Seen(hash, value string) {
	seencheck.SeenDB.Set(hash, value)
	seencheck.SeenCount.Incr(1)
}
