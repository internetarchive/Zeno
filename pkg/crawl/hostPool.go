package crawl

import (
	"sync"

	"github.com/paulbellamy/ratecounter"
)

// Host is an item in Crawl.ActiveHosts,
// it represents an Host and it's occurence in the queue
type Host struct {
	Host  string
	Count *ratecounter.Counter
}

// HostPool holds all the active hosts in the pool
type HostPool struct {
	Mutex *sync.Mutex
	Hosts []Host
}

// IsHostInPool return true if the Host is in the pool
func (pool *HostPool) IsHostInPool(target string) bool {
	for _, host := range pool.Hosts {
		if host.Host == target {
			return true
		}
	}
	return false
}

// Add add a new host to the pool
func (pool *HostPool) Add(host string) {
	newHost := new(Host)

	newHost.Host = host
	newHost.Count = new(ratecounter.Counter)
	newHost.Count.Incr(1)

	pool.Hosts = append(pool.Hosts, *newHost)
}

// Incr increment by 1 the counter of an host in the pool
func (pool *HostPool) Incr(target string) {
	for _, host := range pool.Hosts {
		if host.Host == target {
			host.Count.Incr(1)
		}
	}
}
