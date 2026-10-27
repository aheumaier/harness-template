SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE fx_items RENAME COLUMN name TO title;
