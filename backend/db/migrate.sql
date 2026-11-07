-- Explicit versioned migration, not run by API startup. psql -f backend/db/migrate.sql
\set ON_ERROR_STOP on
BEGIN;
SET LOCAL ROLE watchbrief_owner;
SET LOCAL search_path = public, pg_catalog;
SELECT pg_advisory_xact_lock(530001);
CREATE TABLE IF NOT EXISTS schema_migrations (version bigint PRIMARY KEY);
SELECT NOT EXISTS (SELECT FROM schema_migrations WHERE version = 1) AS apply_v1 \gset
\if :apply_v1
\ir migrations/000001_auth.up.sql
INSERT INTO schema_migrations (version) VALUES (1);
\endif
REVOKE ALL ON users, auth_sessions, schema_migrations FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO watchbrief_api, watchbrief_user_admin;
GRANT SELECT ON users TO watchbrief_api;
GRANT SELECT, INSERT ON auth_sessions TO watchbrief_api;
GRANT INSERT ON users TO watchbrief_user_admin;
COMMIT;
