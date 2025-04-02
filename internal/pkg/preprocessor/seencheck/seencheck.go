package seencheck

import (
	"hash/fnv"
	"path"
	"strconv"
	"sync/atomic"

	"github.com/internetarchive/Zeno/pkg/models"
	"github.com/philippgille/gokv/leveldb"
)

// Seencheck holds the Seencheck database and the seen counter
type Seencheck struct {
	Count *int64
	DB    leveldb.Store
}

var (
	globalSeencheck *Seencheck
)

func Start(jobPath string) (err error) {
	count := int64(0)
	globalSeencheck = new(Seencheck)
	globalSeencheck.Count = &count
	globalSeencheck.DB, err = leveldb.NewStore(leveldb.Options{Path: path.Join(jobPath, "seencheck")})
	return err
}

func Close() {
	globalSeencheck.DB.Close()
}

func isSeen(hash string) (found bool, value string) {
	found, err := globalSeencheck.DB.Get(hash, &value)
	if err != nil {
		panic(err)
	}

	return found, value
}

func seen(hash, value string) {
	globalSeencheck.DB.Set(hash, value)
	atomic.AddInt64(globalSeencheck.Count, 1)
}

// func SeencheckURLs(URLType models.URLType, URLs ...*models.URL) (seencheckedURLs []*models.URL, err error) {
// 	h := fnv.New64a()

// 	for _, URL := range URLs {
// 		_, err = h.Write([]byte(URL.String()))
// 		if err != nil {
// 			return nil, err
// 		}

// 		hash := strconv.FormatUint(h.Sum64(), 10)

// 		found, foundURLType := isSeen(hash)
// 		if !found || (foundURLType == "asset" && URLType == "seed") {
// 			seen(hash, string(URLType))
// 			seencheckedURLs = append(seencheckedURLs, URL)
// 		}

// 		h.Reset()
// 	}

// 	return seencheckedURLs, nil
// }

// SeencheckItem gets the MaxDepth children of the given item and seencheck them locally.
// The items that were seen before will be marked as seen.
// Different from the HQ seencheck, the local seencheck performs seencheck on top level seeds.
func SeencheckItem(item *models.Item) error {
	h := fnv.New64a()

	items, err := item.GetNodesAtLevel(item.GetMaxDepth())
	if err != nil {
		panic(err)
	}

	for i := range items {
		_, err = h.Write([]byte(items[i].GetURL().String()))
		if err != nil {
			return err
		}

		hash := strconv.FormatUint(h.Sum64(), 10)

		var URLType string
		if items[i].IsChild() {
			URLType = "asset"
		} else {
			URLType = "seed"
		}

		found, foundType := isSeen(hash)

		if !found {
			// First time seen: mark and process
			seen(hash, URLType)
			h.Reset()
			continue
		}

		if foundType == "asset" && URLType == "seed" {
			// Promotion: allow processing again as seed
			seen(hash, "seed")
			h.Reset()
			continue
		}

		// All other cases: already seen, skip
		items[i].SetStatus(models.ItemSeen)
		h.Reset()
	}

	return nil
}
