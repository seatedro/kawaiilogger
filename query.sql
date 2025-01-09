-- name: CreateMetrics :one
INSERT INTO metrics (
    keypresses, mouse_clicks, mouse_distance_in, mouse_distance_mi, scroll_steps
) VALUES (
    ?, ?, ?, ?, ?
)
RETURNING id, keypresses, mouse_clicks, mouse_distance_in, mouse_distance_mi, scroll_steps, timestamp;
