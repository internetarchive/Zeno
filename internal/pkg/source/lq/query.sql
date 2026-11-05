-- name: GetFreshURLs :many
SELECT * FROM urls
WHERE status = 'FRESH'
ORDER BY hops ASC, host_rank ASC, timestamp ASC
LIMIT ?;

-- name: GetFreshURLsExcludingHosts :many
SELECT * FROM urls
WHERE status = 'FRESH' AND host NOT IN (sqlc.slice('hosts'))
ORDER BY hops ASC, host_rank ASC, timestamp ASC
LIMIT ?;

-- name: ClaimThisURL :exec
UPDATE urls
SET status = 'CLAIMED', timestamp = strftime('%s', 'now')
WHERE id = ?;

-- name: ResetURL :exec
UPDATE urls
SET status = 'FRESH', timestamp = strftime('%s', 'now')
WHERE id = ?;

-- name: AddURL :execrows
INSERT INTO urls (id, value, via, hops, host, host_rank)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (value) DO NOTHING;

-- name: GetHostRank :one
SELECT CAST(COALESCE((SELECT next FROM host_ranks WHERE host = ? AND hops = ?), 0) AS INTEGER) AS next;

-- name: IncrementHostRank :exec
INSERT INTO host_ranks (host, hops, next)
VALUES (?, ?, 1)
ON CONFLICT (host, hops) DO UPDATE SET next = next + 1;

-- name: DoneURL :exec
UPDATE urls
SET status = 'DONE', timestamp = strftime('%s', 'now')
WHERE id = ?;

-- name: DeleteURL :exec
DELETE FROM urls
WHERE id = ?;

-- name: AddSeedDomain :exec
INSERT INTO seed_domains (domain)
VALUES (?)
ON CONFLICT (domain) DO NOTHING;

-- name: IsSeedDomain :one
SELECT CAST(EXISTS (SELECT 1 FROM seed_domains WHERE domain = ?) AS INTEGER) AS found;
