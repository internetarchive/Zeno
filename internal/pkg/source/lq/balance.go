package lq

import (
	"net/url"
	"sync"
	"time"

	"github.com/internetarchive/Zeno/v2/internal/pkg/archiver"
	"github.com/internetarchive/Zeno/v2/internal/pkg/config"
	"github.com/internetarchive/Zeno/v2/internal/pkg/source/lq/sqlc_model"
)

// rateLimitedHostDelay is the rate limiter wait above which a host is considered
// rate limited (e.g. in a 429 penalty) rather than just paced.
const rateLimitedHostDelay = time.Second

// hostTracker counts the URLs dispatched by the LQ that are still in flight, per host.
type hostTracker struct {
	mu       sync.Mutex
	inflight map[string]int
	byID     map[string]string
}

func newHostTracker() *hostTracker {
	return &hostTracker{
		inflight: make(map[string]int),
		byID:     make(map[string]string),
	}
}

func (t *hostTracker) count(host string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.inflight[host]
}

// atLeast returns the hosts with at least n URLs in flight.
func (t *hostTracker) atLeast(n int) []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	var hosts []string
	for host, count := range t.inflight {
		if count >= n {
			hosts = append(hosts, host)
		}
	}
	return hosts
}

func (t *hostTracker) add(id, host string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.inflight[host]++
	t.byID[id] = host
}

// done releases a URL dispatched by the LQ. Unknown IDs (e.g. input seeds) are ignored.
func (t *hostTracker) done(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	host, ok := t.byID[id]
	if !ok {
		return
	}
	delete(t.byID, id)
	if t.inflight[host] <= 1 {
		delete(t.inflight, host)
		return
	}
	t.inflight[host]--
}

func balancingEnabled() bool {
	return config.Get().MaxInflightPerHost > 0 || config.Get().DeferRateLimitedHosts
}

func hostOf(value string) string {
	u, err := url.Parse(value)
	if err != nil {
		return ""
	}
	return u.Host
}

// getBalancedURLs claims up to batchSize URLs, skipping hosts that already have
// --max-inflight-per-host URLs in flight and, with --defer-rate-limited-hosts,
// hosts the rate limiter is holding back. Skipped URLs stay where they are in the
// queue. It returns the claimed URLs and, when it claimed none, whether that is
// because every host with FRESH URLs is blocked.
func (s *LQ) getBalancedURLs(batchSize int) ([]sqlc_model.Url, bool, error) {
	maxPerHost := config.Get().MaxInflightPerHost
	deferRateLimited := config.Get().DeferRateLimitedHosts

	var excluded []string
	if maxPerHost > 0 {
		excluded = s.hosts.atLeast(maxPerHost)
	}

	inBatch := make(map[string]int)
	blocked := func(host string, claimed bool) bool {
		if host == "" {
			return false // URLs queued without a host are dispatched as before
		}

		if claimed {
			inBatch[host]++
		} else if deferRateLimited && archiver.HostDelay(host) > rateLimitedHostDelay {
			return true
		}

		return maxPerHost > 0 && s.hosts.count(host)+inBatch[host] >= maxPerHost
	}

	URLs, onlyBlocked, err := s.client.getBalanced(s.ctx, batchSize, excluded, blocked)
	if err != nil {
		return nil, false, err
	}

	for i := range URLs {
		s.hosts.add(URLs[i].ID, URLs[i].Host)
	}

	return URLs, onlyBlocked, nil
}
