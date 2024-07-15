-- name: CreateMetrics :one
INSERT INTO metrics (
    keypresses, mouse_clicks
) VALUES (
    $1, $2
)
RETURNING id, keypresses, mouse_clicks, timestamp;

