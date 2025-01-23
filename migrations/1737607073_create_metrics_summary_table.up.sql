CREATE TABLE IF NOT EXISTS metrics (
    id SERIAL PRIMARY KEY,
    keypresses INTEGER NOT NULL,
    mouse_clicks INTEGER NOT NULL,
    mouse_distance_in FLOAT NOT NULL,
    mouse_distance_mi FLOAT NOT NULL,
    scroll_steps INTEGER NOT NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create the summary table
CREATE TABLE metrics_summary (
    id INTEGER PRIMARY KEY,
    total_keypresses BIGINT NOT NULL DEFAULT 0,
    total_mouse_clicks BIGINT NOT NULL DEFAULT 0,
    total_mouse_travel_in DOUBLE PRECISION NOT NULL DEFAULT 0,
    total_mouse_travel_mi DOUBLE PRECISION NOT NULL DEFAULT 0,
    total_scroll_steps BIGINT NOT NULL DEFAULT 0,
    last_updated TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Initialize with a single row
INSERT INTO metrics_summary (id) VALUES (1);

-- Update summary with existing data
UPDATE metrics_summary 
SET 
    total_keypresses = (SELECT COALESCE(SUM(keypresses), 0) FROM metrics),
    total_mouse_clicks = (SELECT COALESCE(SUM(mouse_clicks), 0) FROM metrics),
    total_mouse_travel_in = (SELECT COALESCE(SUM(mouse_distance_in), 0) FROM metrics),
    total_mouse_travel_mi = (SELECT COALESCE(SUM(mouse_distance_mi), 0) FROM metrics),
    total_scroll_steps = (SELECT COALESCE(SUM(scroll_steps), 0) FROM metrics);

-- Create function for the trigger
CREATE OR REPLACE FUNCTION update_metrics_summary_fn()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE metrics_summary 
    SET 
        total_keypresses = total_keypresses + NEW.keypresses,
        total_mouse_clicks = total_mouse_clicks + NEW.mouse_clicks,
        total_mouse_travel_in = total_mouse_travel_in + NEW.mouse_distance_in,
        total_mouse_travel_mi = total_mouse_travel_mi + NEW.mouse_distance_mi,
        total_scroll_steps = total_scroll_steps + NEW.scroll_steps,
        last_updated = CURRENT_TIMESTAMP
    WHERE id = 1;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create the trigger
CREATE TRIGGER update_metrics_summary
    AFTER INSERT ON metrics
    FOR EACH ROW
    EXECUTE FUNCTION update_metrics_summary_fn();
