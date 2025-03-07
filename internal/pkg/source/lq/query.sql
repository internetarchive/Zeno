-- name: GetFreshURLs :many
SELECT * FROM urls
WHERE status = 'FRESH'
LIMIT ?;

-- name: ClaimThisURL :exec
UPDATE urls
SET status = 'CLAIMED', timestamp = strftime('%s', 'now')
WHERE id = ?;

-- name: ResetURL :exec
UPDATE urls
SET status = 'FRESH', timestamp = strftime('%s', 'now')
WHERE id = ?;

-- name: AddURL :exec
INSERT INTO urls (id, value, via, hops)
VALUES (?, ?, ?, ?);

-- name: DoneURL :exec
UPDATE urls
SET status = 'DONE', timestamp = strftime('%s', 'now')
WHERE id = ?;

-- name: DeleteURL :exec
DELETE FROM urls
WHERE id = ?;
