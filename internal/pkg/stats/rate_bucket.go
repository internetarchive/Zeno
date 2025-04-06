package stats

import (
	"sync"
)

type rateBucket struct {
	sync.Mutex
	data map[string]*rate
}

func newRateBucket() *rateBucket {
	return &rateBucket{
		data: make(map[string]*rate),
	}
}

func (rb *rateBucket) get(key string) uint64 {
	rb.Lock()
	defer rb.Unlock()

	if rps, ok := rb.data[key]; ok {
		return rps.get()
	}

	return 0
}

func (rb *rateBucket) getTotal(key string) uint64 {
	rb.Lock()
	defer rb.Unlock()

	if rps, ok := rb.data[key]; ok {
		return rps.getTotal()
	}

	return 0
}

func (rb *rateBucket) getAll() map[string]uint64 {
	rb.Lock()
	defer rb.Unlock()

	m := make(map[string]uint64)
	for k, rps := range rb.data {
		m[k] = rps.get()
	}

	return m
}

func (rb *rateBucket) getAllTotal() map[string]uint64 {
	rb.Lock()
	defer rb.Unlock()

	m := make(map[string]uint64)
	for k, rps := range rb.data {
		m[k] = rps.getTotal()
	}

	return m
}

// getFiltered returns a map of the current stats filtered by the given regex-like pattern.
// For example, if the pattern is "2*", it will return all stats that start with "2".
// If the pattern is "*", it will return all stats.
// If the pattern is "2?", it will return all stats that start with "2" and have one more character.
func (rb *rateBucket) getFiltered(filter string) map[string]uint64 {
	rb.Lock()
	defer rb.Unlock()

	m := make(map[string]uint64)
	for k, rps := range rb.data {
		if match := match(filter, k); match {
			m[k] = rps.get()
		}
	}

	return m
}

func (rb *rateBucket) incr(key string, step uint64) {
	rb.Lock()
	defer rb.Unlock()

	if rps, ok := rb.data[key]; ok {
		rps.incr(step)
		return
	}

	rps := &rate{}
	rps.incr(step)
	rb.data[key] = rps
}

func (rb *rateBucket) reset(key string) {
	rb.Lock()
	defer rb.Unlock()

	if rps, ok := rb.data[key]; ok {
		rps.reset()
	}
}

func (rb *rateBucket) resetAll() {
	rb.Lock()
	defer rb.Unlock()

	for _, rps := range rb.data {
		rps.reset()
	}
}

// match checks if the string s matches the pattern with wildcards
// * matches any sequence of characters
// ? matches any single character
func match(pattern, s string) bool {
	pLen, sLen := len(pattern), len(s)
	pIdx, sIdx := 0, 0
	starIdx, matchIdx := -1, 0

	for sIdx < sLen {
		if pIdx < pLen && (pattern[pIdx] == '?' || pattern[pIdx] == s[sIdx]) {
			pIdx++
			sIdx++
		} else if pIdx < pLen && pattern[pIdx] == '*' {
			starIdx = pIdx
			matchIdx = sIdx
			pIdx++
		} else if starIdx != -1 {
			pIdx = starIdx + 1
			matchIdx++
			sIdx = matchIdx
		} else {
			return false
		}
	}

	for pIdx < pLen && pattern[pIdx] == '*' {
		pIdx++
	}

	return pIdx == pLen
}

func bucketSum(buckets map[string]uint64) uint64 {
	var sum uint64
	for _, v := range buckets {
		sum += v
	}
	return sum
}
