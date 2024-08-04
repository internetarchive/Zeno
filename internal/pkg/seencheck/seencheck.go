package seencheck

import (
	"path"
	"sync/atomic"

	"github.com/philippgille/gokv/leveldb"
)

// Seencheck holds the Seencheck database and the seen counter
type Seencheck struct {
	Count *int64
	DB    leveldb.Store
}

func New(jobPath string) (seencheck *Seencheck, err error) {
	seencheck = new(Seencheck)
	count := int64(0)
	seencheck.Count = &count
	seencheck.DB, err = leveldb.NewStore(leveldb.Options{Path: path.Join(jobPath, "seencheck")})
	if err != nil {
		return seencheck, err
	}

	return seencheck, nil
}

func (seencheck *Seencheck) Close() {
	seencheck.DB.Close()
}

// IsSeen check if the hash is in the seencheck database
func (seencheck *Seencheck) IsSeen(hash string) (found bool, value string) {
	found, err := seencheck.DB.Get(hash, &value)
	if err != nil {
		panic(err)
	}

	return found, value
}

// Seen mark a hash as seen and increment the seen counter
func (seencheck *Seencheck) Seen(hash, value string) {
	seencheck.DB.Set(hash, value)
	atomic.AddInt64(seencheck.Count, 1)
}
