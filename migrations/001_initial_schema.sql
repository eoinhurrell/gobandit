-- Migration 001: Initial schema (representing current state)
-- +migrate Up

CREATE TABLE IF NOT EXISTS tests (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS arms (
    id VARCHAR(255) PRIMARY KEY,
    test_id VARCHAR(255) NOT NULL REFERENCES tests(id),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    successes INTEGER NOT NULL DEFAULT 0,
    failures INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- +migrate Down

DROP TABLE IF EXISTS arms;
DROP TABLE IF EXISTS tests;