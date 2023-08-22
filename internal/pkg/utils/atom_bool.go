package utils

import "sync/atomic"

// TAtomBool define an atomic boolean
type TAtomBool struct{ flag int32 }

// Set set the value of an atomic boolean
func (b *TAtomBool) Set(value bool) {
	var i int32 = 0
	if value {
		i = 1
	}
	atomic.StoreInt32(&(b.flag), int32(i))
}

// Get return the value of an atomic boolean
func (b *TAtomBool) Get() bool {
	if atomic.LoadInt32(&(b.flag)) != 0 {
		return true
	}
	return false
}
