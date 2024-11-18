package seencheck

import (
	"hash/fnv"
	"path"
	"strconv"
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

func (seencheck *Seencheck) SeencheckURL(URL string, URLType string) bool {
	h := fnv.New64a()
	h.Write([]byte(URL))
	hash := strconv.FormatUint(h.Sum64(), 10)

	found, _ := seencheck.IsSeen(hash)
	if found {
		return true
	} else {
		seencheck.Seen(hash, URLType)
		return false
	}
}
