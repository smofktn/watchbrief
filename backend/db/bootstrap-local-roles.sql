-- Initial provisioning for the dedicated local watchbrief database.
-- Separate from goose schema migrations. No passwords or application tables here.
-- Fail and roll back if any role/schema already exists; do not overwrite it.
-- The provisioning command wraps this SQL and password prompts in one transaction.

CREATE ROLE watchbrief_api LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
CREATE ROLE watchbrief_user_creator LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
CREATE ROLE watchbrief_migrator LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;

REVOKE ALL ON DATABASE watchbrief FROM PUBLIC;
GRANT CONNECT ON DATABASE watchbrief TO watchbrief_api, watchbrief_user_creator, watchbrief_migrator;
REVOKE ALL ON SCHEMA public FROM PUBLIC;

CREATE SCHEMA app AUTHORIZATION watchbrief_migrator;
REVOKE ALL ON SCHEMA app FROM PUBLIC;
GRANT USAGE ON SCHEMA app TO watchbrief_api, watchbrief_user_creator;

ALTER ROLE watchbrief_api IN DATABASE watchbrief SET search_path = app, pg_catalog;
ALTER ROLE watchbrief_user_creator IN DATABASE watchbrief SET search_path = app, pg_catalog;
ALTER ROLE watchbrief_migrator IN DATABASE watchbrief SET search_path = app, pg_catalog;
