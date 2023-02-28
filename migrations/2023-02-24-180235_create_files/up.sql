-- Your SQL goes here
CREATE TABLE files (
    absolute_file_path VARCHAR PRIMARY KEY,
    inserted_at TIMESTAMP NOT NULL,
    expiration TIMESTAMP
)