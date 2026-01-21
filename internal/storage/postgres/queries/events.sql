-- name: InsertEvent :one
INSERT INTO system_events (
    public_key,
    policy_id,
    event_type,
    event_data
) VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: GetEventsAfterTimestamp :many
SELECT * FROM system_events WHERE created_at >= $1 ORDER BY created_at ASC;