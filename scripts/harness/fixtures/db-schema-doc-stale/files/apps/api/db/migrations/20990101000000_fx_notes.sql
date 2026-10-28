SET lock_timeout = '2s';
SET statement_timeout = '30s';
CREATE TABLE fx_notes (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  body text NOT NULL
);
