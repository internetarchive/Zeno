package stats

import (
	"testing"
)

func TestRateBucket_Get(t *testing.T) {
	rb := newRateBucket()
	rb.incr("key1", 10)

	if got := rb.get("key1"); got != 10 {
		t.Errorf("get() = %v, want %v", got, 10)
	}

	if got := rb.get("key2"); got != 0 {
		t.Errorf("get() = %v, want %v", got, 0)
	}
}

func TestRateBucket_GetTotal(t *testing.T) {
	rb := newRateBucket()
	rb.incr("key1", 10)

	if got := rb.getTotal("key1"); got != 10 {
		t.Errorf("getTotal() = %v, want %v", got, 10)
	}

	if got := rb.getTotal("key2"); got != 0 {
		t.Errorf("getTotal() = %v, want %v", got, 0)
	}
}

func TestRateBucket_GetAll(t *testing.T) {
	rb := newRateBucket()
	rb.incr("key1", 10)
	rb.incr("key2", 20)

	expected := map[string]uint64{"key1": 10, "key2": 20}
	if got := rb.getAll(); !equalMaps(got, expected) {
		t.Errorf("getAll() = %v, want %v", got, expected)
	}
}

func TestRateBucket_GetAllTotal(t *testing.T) {
	rb := newRateBucket()
	rb.incr("key1", 10)
	rb.incr("key2", 20)

	expected := map[string]uint64{"key1": 10, "key2": 20}
	if got := rb.getAllTotal(); !equalMaps(got, expected) {
		t.Errorf("getAllTotal() = %v, want %v", got, expected)
	}
}

func TestRateBucket_GetFiltered(t *testing.T) {
	rb := newRateBucket()
	rb.incr("key1", 10)
	rb.incr("key2", 20)
	rb.incr("other", 30)

	expected := map[string]uint64{"key1": 10, "key2": 20}
	if got := rb.getFiltered("key*"); !equalMaps(got, expected) {
		t.Errorf("getFiltered() = %v, want %v", got, expected)
	}

	expected = map[string]uint64{"key1": 10}
	if got := rb.getFiltered("key1"); !equalMaps(got, expected) {
		t.Errorf("getFiltered() = %v, want %v", got, expected)
	}

	expected = map[string]uint64{"key2": 20}
	if got := rb.getFiltered("key2"); !equalMaps(got, expected) {
		t.Errorf("getFiltered() = %v, want %v", got, expected)
	}

	expected = map[string]uint64{"other": 30}
	if got := rb.getFiltered("other"); !equalMaps(got, expected) {
		t.Errorf("getFiltered() = %v, want %v", got, expected)
	}

	expected = map[string]uint64{}
	if got := rb.getFiltered("nonexistent"); !equalMaps(got, expected) {
		t.Errorf("getFiltered() = %v, want %v", got, expected)
	}
}

func TestRateBucket_Incr(t *testing.T) {
	rb := newRateBucket()
	rb.incr("key1", 10)

	if got := rb.getTotal("key1"); got != 10 {
		t.Errorf("incr() = %v, want %v", got, 10)
	}

	rb.incr("key1", 5)
	if got := rb.getTotal("key1"); got != 15 {
		t.Errorf("incr() = %v, want %v", got, 15)
	}
}

func TestRateBucket_Reset(t *testing.T) {
	rb := newRateBucket()
	rb.incr("key1", 10)
	rb.reset("key1")

	if got := rb.get("key1"); got != 0 {
		t.Errorf("reset() = %v, want %v", got, 0)
	}
}

func TestRateBucket_ResetAll(t *testing.T) {
	rb := newRateBucket()
	rb.incr("key1", 10)
	rb.incr("key2", 20)
	rb.resetAll()

	if got := rb.get("key1"); got != 0 {
		t.Errorf("resetAll() = %v, want %v", got, 0)
	}

	if got := rb.get("key2"); got != 0 {
		t.Errorf("resetAll() = %v, want %v", got, 0)
	}
}

func TestMatch(t *testing.T) {
	tests := []struct {
		pattern string
		s       string
		want    bool
	}{
		{"*", "anything", true},
		{"", "", true},
		{"", "nonempty", false},
		{"nonempty", "", false},
		{"*", "", true},
		{"?", "", false},
		{"?", "a", true},
		{"?", "ab", false},
		{"*a", "a", true},
		{"*a", "ba", true},
		{"*a", "ab", false},
		{"a*", "a", true},
		{"a*", "ab", true},
		{"a*", "ba", false},
		{"*a*", "a", true},
		{"*a*", "ba", true},
		{"*a*", "ab", true},
		{"*a*", "bab", true},
		{"*a*", "bcb", false},
		{"a*b*c", "abc", true},
		{"a*b*c", "a123b456c", true},
		{"a*b*c", "a123b456d", false},
		{"a*b*c", "ab123c", true},
		{"a*b*c", "a123bc", true},
		{"a*b*c", "ab", false},
		{"a*b*c", "a123b", false},
		{"a*b*c", "b123c", false},
		{"a*b*c", "a123c", false},
		{"a*b*c", "a123b456c789", false},
		{"a**", "a", true},
		{"a**", "ab", true},
		{"a**", "abc", true},
		{"a**", "abcd", true},
		{"a**", "abcde", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.s, func(t *testing.T) {
			if got := match(tt.pattern, tt.s); got != tt.want {
				t.Errorf("match(%q, %q) = %v, want %v", tt.pattern, tt.s, got, tt.want)
			}
		})
	}
}

func equalMaps(a, b map[string]uint64) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
