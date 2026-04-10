CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS jobs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payload     JSONB        NOT NULL,
    status      TEXT         NOT NULL DEFAULT 'pending',  -- pending | running | done | failed
    retry_count INT          NOT NULL DEFAULT 0,
    error       TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_jobs_status ON jobs(status);
