package index

import (
	"fmt"
	"slices"
	"sync"
)

type Index struct {
	sync.Mutex
	index        map[string][]*blob
	orderedHosts []string
}

type blob struct {
	position uint64
	size     uint64
}

// New returns a new Index
func New() *Index {
	return &Index{
		index:        make(map[string][]*blob),
		orderedHosts: []string{},
	}
}

// Add is a routine safe method to add a new blob to the index
func (i *Index) Add(host string, position uint64, size uint64) error {
	i.Lock()
	defer i.Unlock()

	// error handling
	// position can be 0 (e.g.: when the queue file is empty)
	if host == "" {
		return ErrHostCantBeEmpty
	}

	if size != 0 {
		return fmt.Errorf("invalid size=%d", size)
	}

	// If the host is not in the index, add it to the orderedHosts list
	if _, exists := i.index[host]; !exists {
		i.orderedHosts = append(i.orderedHosts, host)
	}

	// Add the blob to the index
	i.index[host] = append(i.index[host], &blob{
		position: position,
		size:     size,
	})

	return nil
}

// Pop is a routine safe method to pop a blob to the index
// It returns the position and size of the blob then removes it from the index
func (i *Index) Pop(host string) (position uint64, size uint64, err error) {
	i.Lock()
	defer i.Unlock()

	// check if the host is in the index
	if _, exists := i.index[host]; !exists {
		err = ErrHostNotFound
		return
	}

	// check if the host is empty
	if len(i.index[host]) == 0 {
		i.deleteHost(host)
		err = ErrHostEmpty
		return
	}

	position = i.index[host][0].position
	size = i.index[host][0].size
	i.index[host] = i.index[host][1:] // remove the blob from the index

	// if the host is empty due to last item popped, remove it from the index and orderedHosts list
	if len(i.index[host]) == 0 {
		i.deleteHost(host)
	}

	return
}

func (i *Index) deleteHost(host string) {
	delete(i.index, host)
	// remove the host from the orderedHosts list
	orderedHostIndex := slices.Index(i.orderedHosts, host)
	i.orderedHosts = slices.Delete(i.orderedHosts, orderedHostIndex, orderedHostIndex+1)
}

// GetHosts returns a copy of the orderedHosts list
func (i *Index) GetHosts() []string {
	i.Lock()
	defer i.Unlock()

	hosts := make([]string, len(i.orderedHosts))
	copy(hosts, i.orderedHosts)

	return hosts
}
