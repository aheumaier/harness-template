-- Bootstrap: database-level settings and extensions only. No business tables (they come with domains).
SET lock_timeout = '2s';
SET statement_timeout = '30s';
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
