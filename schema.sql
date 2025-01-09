CREATE TABLE IF NOT EXISTS metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    keypresses INTEGER NOT NULL,
    mouse_clicks INTEGER NOT NULL,
    mouse_distance_in FLOAT NOT NULL,
    mouse_distance_mi FLOAT NOT NULL,
    scroll_steps INTEGER NOT NULL,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);
