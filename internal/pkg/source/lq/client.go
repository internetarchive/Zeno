package lq

import (
	"context"
	"database/sql"
	_ "embed"
	"path"
	"slices"

	_ "github.com/ncruces/go-sqlite3/driver"

	"github.com/google/uuid"
	"github.com/internetarchive/Zeno/v2/internal/pkg/config"
	"github.com/internetarchive/Zeno/v2/internal/pkg/source/lq/sqlc_model"
)

type lqClient struct {
	dbWrite     *sql.DB
	dbWriteSqlc *sqlc_model.Queries
}

//go:embed schema.sql
var ddl string

func initClient(job string) (*lqClient, error) {
	dbWrite, err := sql.Open("sqlite3", "file:"+path.Join(config.Get().JobPath, "lq.db"))
	if err != nil {
		return nil, err
	}
	dbWrite.SetMaxOpenConns(1)

	if err := addMissingColumns(dbWrite); err != nil {
		logger.Error("error adding columns to lq database", "err", err.Error(), "func", "lq.Init")
		return nil, err
	}

	if _, err := dbWrite.Exec(ddl); err != nil {
		logger.Error("error creating lq database schema", "err", err.Error(), "func", "lq.Init")
		return nil, err
	}

	dbWriteSqlc := sqlc_model.New(dbWrite)

	return &lqClient{
		dbWrite:     dbWrite,
		dbWriteSqlc: dbWriteSqlc,
	}, nil
}

// urlsColumnsAdded are the urls columns added after the table was first
// created, in the order they were added.
var urlsColumnsAdded = []struct{ name, definition string }{
	// URLs queued before it existed have an empty host and are dispatched as before.
	{"host", "host TEXT DEFAULT '' NOT NULL"},
	// URLs queued before it existed have rank 0 and are dequeued first, by timestamp.
	{"host_rank", "host_rank INTEGER NOT NULL DEFAULT 0"},
}

// addMissingColumns adds the columns in urlsColumnsAdded to a urls table created
// before they existed. It does nothing to a new database.
func addMissingColumns(db *sql.DB) error {
	var tables int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'urls'").Scan(&tables); err != nil || tables == 0 {
		return err
	}

	for _, column := range urlsColumnsAdded {
		var found int
		if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('urls') WHERE name = ?", column.name).Scan(&found); err != nil {
			return err
		}
		if found > 0 {
			continue
		}
		if _, err := db.Exec("ALTER TABLE urls ADD COLUMN " + column.definition); err != nil {
			return err
		}
	}
	return nil
}

func (client *lqClient) resetURL(ctx context.Context, seed string) error {
	return client.dbWriteSqlc.ResetURL(ctx, seed)
}

func (client *lqClient) get(ctx context.Context, limit int) ([]sqlc_model.Url, error) {
	tx, err := client.dbWrite.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	qtx := client.dbWriteSqlc.WithTx(tx)

	freshUrls, err := qtx.GetFreshURLs(ctx, int64(limit))
	if err != nil {
		return nil, err
	}

	for _, record := range freshUrls {
		if err = qtx.ClaimThisURL(ctx, record.ID); err != nil {
			logger.Error("error claiming URL", "err", err.Error(), "func", "lq.getURLs", "id", record.ID)
			return nil, err
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return freshUrls, nil
}

// getBalanced claims up to limit FRESH URLs in queue order, skipping the URLs
// whose host is blocked. Hosts in excluded are skipped from the start; blocked is
// asked about every other host found, and again after each URL claimed for it, so
// a host can be blocked once it reaches a per-batch limit. Skipped URLs are left
// untouched. It returns the claimed URLs and, when it claimed none, whether that
// is because every host with FRESH URLs is blocked (rather than an empty queue).
func (client *lqClient) getBalanced(ctx context.Context, limit int, excluded []string, blocked func(host string, claimed bool) bool) ([]sqlc_model.Url, bool, error) {
	tx, err := client.dbWrite.Begin()
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()

	qtx := client.dbWriteSqlc.WithTx(tx)

	var (
		claimed     []sqlc_model.Url
		onlyBlocked bool
	)
	excluded = slices.Clone(excluded)
	for len(claimed) < limit {
		var candidates []sqlc_model.Url
		if len(excluded) == 0 {
			candidates, err = qtx.GetFreshURLs(ctx, int64(limit-len(claimed)))
		} else {
			candidates, err = qtx.GetFreshURLsExcludingHosts(ctx, sqlc_model.GetFreshURLsExcludingHostsParams{
				Hosts: excluded,
				Limit: int64(limit - len(claimed)),
			})
		}
		if err != nil {
			return nil, false, err
		}
		if len(candidates) == 0 {
			break
		}

		for _, record := range candidates {
			// Every candidate either gets claimed or gets its host excluded, so the loop ends
			if slices.Contains(excluded, record.Host) {
				continue
			}

			if blocked(record.Host, false) {
				excluded = append(excluded, record.Host)
				continue
			}

			if err = qtx.ClaimThisURL(ctx, record.ID); err != nil {
				logger.Error("error claiming URL", "err", err.Error(), "func", "lq.getBalanced", "id", record.ID)
				return nil, false, err
			}
			claimed = append(claimed, record)

			if blocked(record.Host, true) {
				excluded = append(excluded, record.Host)
			}
		}
	}

	if len(claimed) == 0 && len(excluded) > 0 {
		// Tell an empty queue apart from one where every host is blocked
		fresh, err := qtx.GetFreshURLs(ctx, 1)
		if err != nil {
			return nil, false, err
		}
		onlyBlocked = len(fresh) > 0
	}

	if err = tx.Commit(); err != nil {
		return nil, false, err
	}

	return claimed, onlyBlocked, nil
}

func (client *lqClient) add(ctx context.Context, urls []sqlc_model.Url, bypassSeencheck bool) error {
	tx, err := client.dbWrite.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := client.dbWriteSqlc.WithTx(tx)

	for _, url := range urls {
		if url.ID == "" {
			url.ID = uuid.New().String()
		}
		host := hostOf(url.Value)

		// host_rank is the URL's position among its host's URLs at the same hop, so
		// dequeueing by (hops, host_rank) takes hosts in turn: every host's first URL,
		// then every host's second URL... A host whose URLs are queued late is served
		// right away instead of after every URL queued before it.
		var rank int64
		if host != "" {
			rank, err = qtx.GetHostRank(ctx, sqlc_model.GetHostRankParams{Host: host, Hops: int64(url.Hops)})
			if err != nil {
				logger.Error("error getting host rank", "err", err.Error(), "func", "lq.Add", "host", host)
				return err
			}
		}

		added, err := qtx.AddURL(ctx, sqlc_model.AddURLParams{
			ID:       url.ID,
			Value:    url.Value,
			Via:      url.Via,
			Hops:     int64(url.Hops),
			Host:     host,
			HostRank: rank,
		})
		if err != nil {
			logger.Error("error adding URL", "err", err.Error(), "func", "lq.Add", "value", url.Value, "via", url.Via)
			return err
		}
		if added == 0 {
			logger.Debug("URL.Value already exists in LQ", "value", url.Value, "via", url.Via)
			continue
		}

		// Only URLs actually queued take a rank, so duplicates don't push a host back.
		if host != "" {
			if err = qtx.IncrementHostRank(ctx, sqlc_model.IncrementHostRankParams{Host: host, Hops: int64(url.Hops)}); err != nil {
				logger.Error("error incrementing host rank", "err", err.Error(), "func", "lq.Add", "host", host)
				return err
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (client *lqClient) delete(ctx context.Context, urls []sqlc_model.Url, bypassSeencheck bool) error {
	tx, err := client.dbWrite.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := client.dbWriteSqlc.WithTx(tx)

	for _, url := range urls {
		err = qtx.DeleteURL(ctx, url.ID)
		if err != nil {
			logger.Error("error deleting URL", "err", err.Error(), "func", "lq.Delete", "id", url.ID)
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}
