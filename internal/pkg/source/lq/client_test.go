package lq

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/internetarchive/Zeno/v2/internal/pkg/log"
	"github.com/internetarchive/Zeno/v2/internal/pkg/source/lq/sqlc_model"
)

func newTestClient(t *testing.T) *lqClient {
	t.Helper()

	if logger == nil {
		logger = log.NewFieldedLogger(&log.Fields{"component": "lq"}) // set by Start in a crawl
	}

	db, err := sql.Open("sqlite3", "file:"+filepath.Join(t.TempDir(), "lq.db"))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(ddl); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	return &lqClient{
		dbWrite:     db,
		dbWriteSqlc: sqlc_model.New(db),
	}
}

func TestGetFreshURLsOrder(t *testing.T) {
	t.Run("with schema indexes", func(t *testing.T) {
		testGetFreshURLsOrder(t, newTestClient(t))
	})

	// Scanning urls_status_hops_host_rank_timestamp happens to return rows in order even
	// without an ORDER BY, so drop it to check the query itself is ordered.
	t.Run("without ordered index", func(t *testing.T) {
		client := newTestClient(t)
		if _, err := client.dbWrite.Exec("DROP INDEX urls_status_hops_host_rank_timestamp"); err != nil {
			t.Fatalf("failed to drop index: %v", err)
		}
		testGetFreshURLsOrder(t, client)
	})
}

func testGetFreshURLsOrder(t *testing.T, client *lqClient) {
	// Inserted in an order that differs from the expected dequeue order, so
	// that rowid (insertion) order cannot accidentally satisfy the test.
	rows := []struct {
		id        string
		hops      int
		timestamp int
		status    string
	}{
		{"hop2-old", 2, 100, "FRESH"},
		{"hop1-new", 1, 300, "FRESH"},
		{"hop0-new", 0, 300, "FRESH"},
		{"hop1-old", 1, 100, "FRESH"},
		{"hop0-claimed", 0, 50, "CLAIMED"},
		{"hop0-old", 0, 100, "FRESH"},
		{"hop0-done", 0, 50, "DONE"},
		{"hop1-mid", 1, 200, "FRESH"},
	}
	for _, r := range rows {
		_, err := client.dbWrite.Exec(
			"INSERT INTO urls (id, value, hops, status, timestamp) VALUES (?, ?, ?, ?, ?)",
			r.id, "https://example.com/"+r.id, r.hops, r.status, r.timestamp,
		)
		if err != nil {
			t.Fatalf("failed to insert %s: %v", r.id, err)
		}
	}

	expected := []string{"hop0-old", "hop0-new", "hop1-old", "hop1-mid", "hop1-new", "hop2-old"}

	urls, err := client.dbWriteSqlc.GetFreshURLs(context.Background(), int64(len(rows)))
	if err != nil {
		t.Fatalf("GetFreshURLs failed: %v", err)
	}
	if len(urls) != len(expected) {
		t.Fatalf("expected %d FRESH URLs, got %d", len(expected), len(urls))
	}
	for i, url := range urls {
		if url.ID != expected[i] {
			t.Errorf("position %d: expected %s, got %s (hops=%d, timestamp=%d)", i, expected[i], url.ID, url.Hops, url.Timestamp)
		}
	}

	// get() must claim the lowest hops first, leaving the rest FRESH.
	claimed, err := client.get(context.Background(), 2)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if len(claimed) != 2 || claimed[0].ID != "hop0-old" || claimed[1].ID != "hop0-new" {
		t.Fatalf("expected get to claim [hop0-old hop0-new], got %v", claimed)
	}

	remaining, err := client.dbWriteSqlc.GetFreshURLs(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetFreshURLs failed: %v", err)
	}
	if len(remaining) != 1 || remaining[0].ID != "hop1-old" {
		t.Fatalf("expected hop1-old to be next, got %v", remaining)
	}
}

func TestGetFreshURLsQueryPlan(t *testing.T) {
	client := newTestClient(t)

	// Same query as GetFreshURLs in query.sql. Dequeueing must not sort every
	// FRESH row, which happens when no index covers (status, hops, host_rank, timestamp).
	rows, err := client.dbWrite.Query(`EXPLAIN QUERY PLAN
SELECT id, value, via, hops, status, timestamp FROM urls
WHERE status = 'FRESH'
ORDER BY hops ASC, host_rank ASC, timestamp ASC
LIMIT ?`, 10)
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
		t.Errorf("GetFreshURLs sorts in a temp b-tree, plan:\n%s", joined)
	}
	if !strings.Contains(joined, "urls_status_hops_host_rank_timestamp") {
		t.Errorf("GetFreshURLs does not use urls_status_hops_host_rank_timestamp, plan:\n%s", joined)
	}
}

func TestAddRotatesHosts(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	add := func(hops int, values ...string) {
		t.Helper()
		var urls []sqlc_model.Url
		for _, v := range values {
			urls = append(urls, sqlc_model.Url{Value: v, Hops: int64(hops)})
		}
		if err := client.add(ctx, urls, false); err != nil {
			t.Fatalf("add failed: %v", err)
		}
	}

	// a.com queues a long run of hop-1 URLs first, b.com and c.com come later.
	add(1, "https://a.com/1", "https://a.com/2", "https://a.com/3", "https://a.com/4")
	add(1, "https://b.com/1", "https://b.com/2")
	add(1, "https://a.com/2") // duplicate: must not take a rank
	add(1, "https://c.com/1")
	add(0, "https://a.com/") // ranks are per hop
	add(1, "https://a.com/5")

	urls, err := client.dbWriteSqlc.GetFreshURLs(ctx, 20)
	if err != nil {
		t.Fatalf("GetFreshURLs failed: %v", err)
	}
	var got []string
	for _, u := range urls {
		got = append(got, u.Value)
	}
	want := []string{
		"https://a.com/",
		"https://a.com/1", "https://b.com/1", "https://c.com/1",
		"https://a.com/2", "https://b.com/2",
		"https://a.com/3",
		"https://a.com/4",
		"https://a.com/5",
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("dequeue order:\n got %v\nwant %v", got, want)
	}

	var rank int64
	if err := client.dbWrite.QueryRow("SELECT host_rank FROM urls WHERE value = 'https://a.com/5'").Scan(&rank); err != nil {
		t.Fatalf("failed to read host_rank: %v", err)
	}
	if rank != 4 {
		t.Errorf("a.com/5 has host_rank %d, want 4 (the duplicate must not take a rank)", rank)
	}
}
