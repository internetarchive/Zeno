CREATE TABLE IF NOT EXISTS urls (
    id TEXT NOT NULL PRIMARY KEY,
    value TEXT NOT NULL,
    via TEXT DEFAULT '' NOT NULL,
    hops INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'FRESH' CHECK (status IN ('FRESH', 'CLAIMED', 'DONE')),
    timestamp INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    host TEXT DEFAULT '' NOT NULL,
    host_rank INTEGER NOT NULL DEFAULT 0 -- position of this URL among its host's URLs at the same hop
);
CREATE TABLE IF NOT EXISTS host_ranks (
    host TEXT NOT NULL,
    hops INTEGER NOT NULL,
    next INTEGER NOT NULL DEFAULT 0, -- host_rank of the next URL queued for this host at this hop
    PRIMARY KEY (host, hops)
);
CREATE TABLE IF NOT EXISTS seed_domains ( -- with --lq-seed-domains-first, registrable domains of the URLs queued at hop 0
    domain TEXT NOT NULL PRIMARY KEY
);
CREATE UNIQUE INDEX IF NOT EXISTS urls_value ON urls (value); -- for deduplication
CREATE INDEX IF NOT EXISTS urls_status ON urls (status); -- for queueing
DROP INDEX IF EXISTS urls_status_hops_timestamp;
CREATE INDEX IF NOT EXISTS urls_status_hops_host_rank_timestamp ON urls (status, hops ASC, host_rank ASC, timestamp ASC); -- for dequeueing FRESH URLs by crawl depth, taking hosts in turn, without a sort
