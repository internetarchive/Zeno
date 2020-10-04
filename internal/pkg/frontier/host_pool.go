package frontier

import (
	"sync"

	"github.com/paulbellamy/ratecounter"
)

// HostPool holds all the active hosts in the pool
type HostPool struct {
	*sync.Mutex
	Hosts map[string]*ratecounter.Counter
}

// IsHostInPool return true if the Host is in the pool
func (pool *HostPool) IsHostInPool(host string) bool {
	pool.Lock()
	if _, ok := pool.Hosts[host]; ok {
		pool.Unlock()
		return true
	}
	pool.Unlock()
	return false
}

// DeleteEmptyHosts remove all the hosts that have a count
// of zero from the hosts pool
func (pool *HostPool) DeleteEmptyHosts() {
	pool.Lock()
	for host, hostCount := range pool.Hosts {
		if hostCount.Value() <= 0 {
			delete(pool.Hosts, host)
		}
	}
	pool.Unlock()
}

// Incr increment by 1 the counter of an host in the pool
func (pool *HostPool) Incr(host string) {
	pool.Lock()
	if _, ok := pool.Hosts[host]; !ok {
		pool.Hosts[host] = new(ratecounter.Counter)
	}
	pool.Hosts[host].Incr(1)
	pool.Unlock()
}
