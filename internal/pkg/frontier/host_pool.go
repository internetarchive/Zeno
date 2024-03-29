package frontier

import "github.com/sirupsen/logrus"

type PoolItem struct {
	TotalCount  uint64
	ActiveCount uint64
}

// IsHostInPool return true if the Host is in the pool
func (f *Frontier) IsHostInPool(host string) bool {
	if _, ok := f.HostPool.Load(host); ok {
		return true
	}
	return false
}

// Incr increment by 1 the counter of an host in the pool
func (f *Frontier) IncrHost(host string) {
	for {
		v, ok := f.HostPool.Load(host)
		if !ok {
			f.HostPool.Store(host, PoolItem{1, 0})
			return
		}

		swapped := f.HostPool.CompareAndSwap(host, v, PoolItem{
			v.(PoolItem).TotalCount + 1,
			v.(PoolItem).ActiveCount,
		})

		if !swapped {
			f.LoggingChan <- &FrontierLogMessage{
				Fields: logrus.Fields{
					"host": host,
				},
				Message: "unable to swap host pool item for host increase",
				Level:   logrus.ErrorLevel,
			}

			continue
		}

		return
	}
}

func (f *Frontier) IncrHostActive(host string) {
	for {
		v, ok := f.HostPool.Load(host)
		if !ok {
			f.HostPool.Store(host, PoolItem{1, 1})
			return
		}

		swapped := f.HostPool.CompareAndSwap(host, v, PoolItem{
			v.(PoolItem).TotalCount + 1,
			v.(PoolItem).ActiveCount + 1,
		})

		if !swapped {
			f.LoggingChan <- &FrontierLogMessage{
				Fields: logrus.Fields{
					"host": host,
				},
				Message: "unable to swap host pool item for active host increase",
				Level:   logrus.ErrorLevel,
			}

			continue
		}

		return
	}
}

// Decr decrement by 1 the counter of an host in the pool
func (f *Frontier) DecrHost(host string) {
	for {
		v, ok := f.HostPool.Load(host)
		if !ok {
			return
		}

		swapped := f.HostPool.CompareAndSwap(host, v, PoolItem{
			v.(PoolItem).TotalCount - 1,
			v.(PoolItem).ActiveCount,
		})

		if !swapped {
			f.LoggingChan <- &FrontierLogMessage{
				Fields: logrus.Fields{
					"host": host,
				},
				Message: "unable to swap host pool item for host decrease",
				Level:   logrus.ErrorLevel,
			}

			continue
		}

		return
	}
}

func (f *Frontier) DecrHostActive(host string) {
	for {
		v, ok := f.HostPool.Load(host)
		if !ok {
			return
		}

		swapped := f.HostPool.CompareAndSwap(host, v, PoolItem{
			v.(PoolItem).TotalCount,
			v.(PoolItem).ActiveCount - 1,
		})

		if !swapped {
			f.LoggingChan <- &FrontierLogMessage{
				Fields: logrus.Fields{
					"host": host,
				},
				Message: "unable to swap host pool item for active host decrease",
				Level:   logrus.ErrorLevel,
			}

			continue
		}

		if v.(PoolItem).TotalCount == 0 && v.(PoolItem).ActiveCount == 0 {
			f.HostPool.Delete(host)
		}

		return
	}
}

// GetCount return the counter of the key
func (f *Frontier) GetHostCount(host string) (value int) {
	v, ok := f.HostPool.Load(host)
	if !ok {
		return 0
	}

	return int(v.(PoolItem).TotalCount)
}

func (f *Frontier) GetActiveHostCount(host string) (value int) {
	v, ok := f.HostPool.Load(host)
	if !ok {
		return 0
	}

	return int(v.(PoolItem).ActiveCount)
}

func (f *Frontier) GetHostsCount() (value int64) {
	var count int64

	f.HostPool.Range(func(host any, count any) bool {
		value++
		return true
	})

	return count
}
