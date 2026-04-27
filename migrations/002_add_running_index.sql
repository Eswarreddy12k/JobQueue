CREATE INDEX IF NOT EXISTS idx_jobs_running ON jobs(updated_at) WHERE status = 'running';
