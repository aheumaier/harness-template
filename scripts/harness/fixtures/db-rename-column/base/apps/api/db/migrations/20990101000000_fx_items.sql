SET lock_timeout = '2s';
SET statement_timeout = '30s';
CREATE TABLE fx_items (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name text NOT NULL
);
