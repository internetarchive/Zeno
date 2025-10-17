package deadhosts

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"time"
)

// Manager tracks hosts that consistently deny connections
// Uses sync.RWMutex instead of sync.Map because we need complex operations
// like iterating through all entries for cleanup and age checks
type Manager struct {
	mu            sync.RWMutex
	deadHosts     map[string]*hostInfo
	enabled       bool
	maxFailures   int           // number of failures before marking as dead
	refreshPeriod time.Duration // how often to clean up the cache
	maxAge        time.Duration // how long to keep a host as dead
	ctx           context.Context
	cancel        context.CancelFunc
	done          chan struct{}
}

type hostInfo struct {
	failures  int
	firstSeen time.Time
	lastSeen  time.Time
}

// NewManager creates a new dead hosts manager
func NewManager(ctx context.Context, enabled bool, maxFailures int, refreshPeriod, maxAge time.Duration) *Manager {
	if !enabled {
		return &Manager{enabled: false}
	}

	managerCtx, cancel := context.WithCancel(ctx)

	m := &Manager{
		deadHosts:     make(map[string]*hostInfo),
		enabled:       enabled,
		maxFailures:   maxFailures,
		refreshPeriod: refreshPeriod,
		maxAge:        maxAge,
		ctx:           managerCtx,
		cancel:        cancel,
		done:          make(chan struct{}),
	}

	go m.cleanupLoop()
	return m
}

// Close stops the dead hosts manager
func (m *Manager) Close() {
	if !m.enabled {
		return
	}
	m.cancel()
	<-m.done
}

// IsDeadHost checks if a host is marked as dead
func (m *Manager) IsDeadHost(host string) bool {
	if !m.enabled {
		return false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	info, exists := m.deadHosts[host]
	if !exists {
		return false
	}

	return info.failures >= m.maxFailures
}

// RecordFailure records a failure for a host
func (m *Manager) RecordFailure(host string, err error) {
	if !m.enabled || !isDeadHostError(err) {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	info, exists := m.deadHosts[host]
	if !exists {
		info = &hostInfo{
			firstSeen: now,
		}
		m.deadHosts[host] = info
	}

	info.failures++
	info.lastSeen = now
}

// RecordSuccess records a successful connection for a host
func (m *Manager) RecordSuccess(host string) {
	if !m.enabled {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove from dead hosts on success
	delete(m.deadHosts, host)
}

// isDeadHostError determines if an error indicates a dead host
func isDeadHostError(err error) bool {
	if err == nil {
		return false
	}

	// First try to unwrap and check for specific error types
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		// Check the underlying error in OpError
		if opErr.Op == "dial" {
			return true // Dial errors are usually connection failures
		}
		if opErr.Op == "read" && netErr.Timeout() {
			return true // Read timeouts
		}
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true // DNS errors indicate unreachable hosts
	}

	// Fall back to string checking for cases not covered by error types
	errStr := err.Error()
	
	// Network-level failures that indicate dead hosts
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "network is unreachable") ||
		strings.Contains(errStr, "host is unreachable") ||
		strings.Contains(errStr, "no route to host") ||
		strings.Contains(errStr, "dns lookup failed") ||
		strings.Contains(errStr, "server misbehaving") {
		return true
	}

	return false
}

// cleanupLoop periodically removes old entries from the dead hosts cache
func (m *Manager) cleanupLoop() {
	defer close(m.done)

	ticker := time.NewTicker(m.refreshPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanup()
		case <-m.ctx.Done():
			return
		}
	}
}

// cleanup removes old entries from the dead hosts cache
func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for host, info := range m.deadHosts {
		if now.Sub(info.firstSeen) > m.maxAge {
			delete(m.deadHosts, host)
		}
	}
}

// GetStats returns statistics about the dead hosts cache
func (m *Manager) GetStats() (deadCount, totalTracked int) {
	if !m.enabled {
		return 0, 0
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	totalTracked = len(m.deadHosts)
	for _, info := range m.deadHosts {
		if info.failures >= m.maxFailures {
			deadCount++
		}
	}

	return deadCount, totalTracked
}
