package lq

import (
	"context"
	"database/sql"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/internetarchive/Zeno/v2/internal/pkg/source/lq/sqlc_model"
)

// insertHostURLs inserts FRESH hop-1 URLs named <host>/<n>, with increasing
// timestamps in the order given.
func insertHostURLs(t *testing.T, client *lqClient, hosts ...string) {
	t.Helper()

	seen := make(map[string]int)
	for i, host := range hosts {
		seen[host]++
		id := host + "/" + string(rune('0'+seen[host]))
		_, err := client.dbWrite.Exec(
			"INSERT INTO urls (id, value, hops, timestamp, host) VALUES (?, ?, 1, ?, ?)",
			id, "https://"+id, 100+i, host,
		)
		if err != nil {
			t.Fatalf("failed to insert %s: %v", id, err)
		}
	}
}

func ids(urls []sqlc_model.Url) []string {
	var out []string
	for _, u := range urls {
		out = append(out, u.ID)
	}
	return out
}

// onePerHost blocks a host once one of its URLs is claimed in the batch.
func onePerHost(host string, claimed bool) bool {
	return claimed
}

func TestGetBalanced(t *testing.T) {
	tests := []struct {
		name            string
		limit           int
		excluded        []string
		blocked         func(host string, claimed bool) bool
		wantClaimed     []string
		wantOnlyBlocked bool
		wantFresh       []string
	}{
		{
			name:            "one per host reaches past a long run of one host",
			limit:           10,
			blocked:         onePerHost,
			wantClaimed:     []string{"a.com/1", "b.com/1", "c.com/1"},
			wantOnlyBlocked: false,
			wantFresh:       []string{"a.com/2", "a.com/3", "a.com/4", "b.com/2"},
		},
		{
			name:            "excluded hosts are skipped from the start",
			limit:           10,
			excluded:        []string{"a.com"},
			blocked:         onePerHost,
			wantClaimed:     []string{"b.com/1", "c.com/1"},
			wantOnlyBlocked: false,
			wantFresh:       []string{"a.com/1", "a.com/2", "a.com/3", "a.com/4", "b.com/2"},
		},
		{
			name:        "limit is respected",
			limit:       2,
			blocked:     onePerHost,
			wantClaimed: []string{"a.com/1", "b.com/1"},
			wantFresh:   []string{"a.com/2", "a.com/3", "a.com/4", "b.com/2", "c.com/1"},
		},
		{
			name:            "blocked before claiming",
			limit:           10,
			blocked:         func(host string, claimed bool) bool { return host == "b.com" },
			wantClaimed:     []string{"a.com/1", "a.com/2", "a.com/3", "a.com/4", "c.com/1"},
			wantOnlyBlocked: false,
			wantFresh:       []string{"b.com/1", "b.com/2"},
		},
		{
			name:            "every host blocked",
			limit:           10,
			blocked:         func(host string, claimed bool) bool { return true },
			wantClaimed:     nil,
			wantOnlyBlocked: true,
			wantFresh:       []string{"a.com/1", "a.com/2", "a.com/3", "a.com/4", "b.com/1", "b.com/2", "c.com/1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t)
			insertHostURLs(t, client, "a.com", "a.com", "a.com", "a.com", "b.com", "b.com", "c.com")

			claimed, onlyBlocked, err := client.getBalanced(context.Background(), tt.limit, tt.excluded, tt.blocked)
			if err != nil {
				t.Fatalf("getBalanced failed: %v", err)
			}
			if !slices.Equal(ids(claimed), tt.wantClaimed) {
				t.Errorf("claimed %v, want %v", ids(claimed), tt.wantClaimed)
			}
			if onlyBlocked != tt.wantOnlyBlocked {
				t.Errorf("onlyBlocked = %v, want %v", onlyBlocked, tt.wantOnlyBlocked)
			}

			// Skipped URLs are left FRESH and in their original order
			fresh, err := client.dbWriteSqlc.GetFreshURLs(context.Background(), 100)
			if err != nil {
				t.Fatalf("GetFreshURLs failed: %v", err)
			}
			if !slices.Equal(ids(fresh), tt.wantFresh) {
				t.Errorf("FRESH after getBalanced %v, want %v", ids(fresh), tt.wantFresh)
			}
		})
	}
}

func TestGetBalancedEmptyQueue(t *testing.T) {
	client := newTestClient(t)

	claimed, onlyBlocked, err := client.getBalanced(context.Background(), 10, []string{"a.com"}, onePerHost)
	if err != nil {
		t.Fatalf("getBalanced failed: %v", err)
	}
	if len(claimed) != 0 || onlyBlocked {
		t.Errorf("expected nothing claimed and onlyBlocked=false on an empty queue, got %v, onlyBlocked=%v", ids(claimed), onlyBlocked)
	}
}

func TestAddSetsHost(t *testing.T) {
	client := newTestClient(t)

	err := client.add(context.Background(), []sqlc_model.Url{
		{ID: "1", Value: "https://www.example.com:8080/a"},
		{ID: "2", Value: "https://example.org/b"},
	}, false)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}

	urls, err := client.dbWriteSqlc.GetFreshURLs(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetFreshURLs failed: %v", err)
	}
	hosts := make(map[string]string)
	for _, u := range urls {
		hosts[u.ID] = u.Host
	}
	if hosts["1"] != "www.example.com:8080" || hosts["2"] != "example.org" {
		t.Errorf("unexpected hosts: %v", hosts)
	}
}

func TestHostTracker(t *testing.T) {
	tr := newHostTracker()
	tr.add("1", "a.com")
	tr.add("2", "a.com")
	tr.add("3", "b.com")

	if got := tr.atLeast(2); !slices.Equal(got, []string{"a.com"}) {
		t.Errorf("atLeast(2) = %v, want [a.com]", got)
	}

	tr.done("1")
	tr.done("unknown") // e.g. an input seed, not dispatched by the LQ
	tr.done("1")       // already released

	if got := tr.count("a.com"); got != 1 {
		t.Errorf("count(a.com) = %d, want 1", got)
	}
	if got := tr.atLeast(2); len(got) != 0 {
		t.Errorf("atLeast(2) = %v, want none", got)
	}

	tr.done("2")
	tr.done("3")
	if len(tr.inflight) != 0 || len(tr.byID) != 0 {
		t.Errorf("expected tracker to be empty, got %v %v", tr.inflight, tr.byID)
	}
}

func TestGetFreshURLsExcludingHostsQueryPlan(t *testing.T) {
	client := newTestClient(t)

	// Same query as GetFreshURLsExcludingHosts in query.sql, with two hosts.
	rows, err := client.dbWrite.Query(`EXPLAIN QUERY PLAN
SELECT id, value, via, hops, status, timestamp, host FROM urls
WHERE status = 'FRESH' AND host NOT IN (?, ?)
ORDER BY hops ASC, timestamp ASC
LIMIT ?`, "a.com", "b.com", 10)
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN failed: %v", err)
	}
	defer rows.Close()

	var plan []string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatalf("failed to scan query plan: %v", err)
		}
		plan = append(plan, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("failed to read query plan: %v", err)
	}

	joined := strings.Join(plan, "\n")
	if strings.Contains(joined, "TEMP B-TREE") {
		t.Errorf("GetFreshURLsExcludingHosts sorts in a temp b-tree, plan:\n%s", joined)
	}
	if !strings.Contains(joined, "urls_status_hops_timestamp") {
		t.Errorf("GetFreshURLsExcludingHosts does not use urls_status_hops_timestamp, plan:\n%s", joined)
	}
}

func TestAddHostColumnToOldDatabase(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+filepath.Join(t.TempDir(), "lq.db"))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()

	// The urls table as created before the host column existed
	oldSchema := strings.Replace(ddl, ",\n    host TEXT DEFAULT '' NOT NULL", "", 1)
	if oldSchema == ddl {
		t.Fatal("failed to derive the old schema from schema.sql")
	}
	if _, err := db.Exec(oldSchema); err != nil {
		t.Fatalf("failed to create old schema: %v", err)
	}
	if _, err := db.Exec("INSERT INTO urls (id, value) VALUES ('old', 'https://example.com/old')"); err != nil {
		t.Fatalf("failed to insert old row: %v", err)
	}

	for range 2 { // must be idempotent
		if err := addHostColumn(db); err != nil {
			t.Fatalf("addHostColumn failed: %v", err)
		}
		if _, err := db.Exec(ddl); err != nil {
			t.Fatalf("failed to apply schema after migration: %v", err)
		}
	}

	client := &lqClient{dbWrite: db, dbWriteSqlc: sqlc_model.New(db)}
	if err := client.add(context.Background(), []sqlc_model.Url{{ID: "new", Value: "https://example.org/new"}}, false); err != nil {
		t.Fatalf("add after migration failed: %v", err)
	}

	urls, err := client.dbWriteSqlc.GetFreshURLs(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetFreshURLs failed: %v", err)
	}
	hosts := make(map[string]string)
	for _, u := range urls {
		hosts[u.ID] = u.Host
	}
	if hosts["old"] != "" || hosts["new"] != "example.org" {
		t.Errorf("unexpected hosts after migration: %v", hosts)
	}
}

func TestAddHostColumnToNewDatabase(t *testing.T) {
	client := newTestClient(t) // schema already has the host column
	if err := addHostColumn(client.dbWrite); err != nil {
		t.Fatalf("addHostColumn failed on a current schema: %v", err)
	}
}
