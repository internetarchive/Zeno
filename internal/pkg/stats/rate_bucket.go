package stats

import (
	"sync"
)

type rateBucket struct {
	data sync.Map // key: string, value: *rate
}

func newRateBucket() *rateBucket {
	return &rateBucket{}
}

func (rb *rateBucket) get(key string) uint64 {
	if v, ok := rb.data.Load(key); ok {
		return v.(*rate).get()
	}
	return 0
}

func (rb *rateBucket) getTotal(key string) uint64 {
	if v, ok := rb.data.Load(key); ok {
		return v.(*rate).getTotal()
	}
	return 0
}

func (rb *rateBucket) getAll() map[string]uint64 {
	m := make(map[string]uint64)
	rb.data.Range(func(k, v any) bool {
		m[k.(string)] = v.(*rate).get()
		return true
	})
	return m
}

func (rb *rateBucket) getAllTotal() map[string]uint64 {
	m := make(map[string]uint64)
	rb.data.Range(func(k, v any) bool {
		m[k.(string)] = v.(*rate).getTotal()
		return true
	})
	return m
}

func (rb *rateBucket) getFiltered(filter string) map[string]uint64 {
	m := make(map[string]uint64)
	rb.data.Range(func(k, v any) bool {
		key := k.(string)
		if match(filter, key) {
			m[key] = v.(*rate).get()
		}
		return true
	})
	return m
}

func (rb *rateBucket) incr(key string, step uint64) {
	if v, ok := rb.data.Load(key); ok {
		v.(*rate).incr(step)
		return
	}
	r := &rate{}
	r.incr(step)
	rb.data.Store(key, r)
}

func (rb *rateBucket) reset(key string) {
	if v, ok := rb.data.Load(key); ok {
		v.(*rate).reset()
	}
}

func (rb *rateBucket) resetAll() {
	rb.data.Range(func(_, v any) bool {
		v.(*rate).reset()
		return true
	})
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
