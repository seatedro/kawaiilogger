-- name: CreateMetrics :one
INSERT INTO metrics (
    keypresses, mouse_clicks, mouse_distance, scroll_distance
) VALUES (
    $1, $2, $3, $4
)
RETURNING id, keypresses, mouse_clicks, mouse_distance, scroll_distance, timestamp;

