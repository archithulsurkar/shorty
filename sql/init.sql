-- Create the URLs table
CREATE TABLE IF NOT EXISTS urls (
    id SERIAL PRIMARY KEY,
    short_code VARCHAR(10) UNIQUE NOT NULL,
    original_url TEXT NOT NULL,
    clicks INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create index on short_code for faster lookups
CREATE INDEX IF NOT EXISTS idx_urls_short_code ON urls(short_code);

-- Create index on original_url to check for duplicates
CREATE INDEX IF NOT EXISTS idx_urls_original_url ON urls(original_url);
