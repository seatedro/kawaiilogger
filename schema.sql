CREATE TABLE IF NOT EXISTS metrics (
    id SERIAL PRIMARY KEY,
    keypresses INTEGER NOT NULL,
    mouse_clicks INTEGER NOT NULL,
    mouse_distance FLOAT NOT NULL,
    scroll_distance FLOAT NOT NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
