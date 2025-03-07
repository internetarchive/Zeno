CREATE TABLE IF NOT EXISTS urls (
    id TEXT NOT NULL PRIMARY KEY,
    value TEXT NOT NULL,
    via TEXT DEFAULT '' NOT NULL,
    hops INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'FRESH' CHECK (status IN ('FRESH', 'CLAIMED', 'DONE')),
    timestamp INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS urls_value ON urls (value); -- for deduplication
CREATE INDEX IF NOT EXISTS urls_status ON urls (status); -- for queueing
