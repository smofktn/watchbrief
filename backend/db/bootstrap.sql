-- Run once as the bootstrap superuser in the dedicated watchbrief database.
-- Passwords are entered using psql's hidden prompt, never SQL literals.
\set ON_ERROR_STOP on
CREATE ROLE watchbrief_owner NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
CREATE ROLE watchbrief_migrator LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
CREATE ROLE watchbrief_api LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
CREATE ROLE watchbrief_user_admin LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
GRANT watchbrief_owner TO watchbrief_migrator;
REVOKE ALL ON DATABASE watchbrief FROM PUBLIC;
GRANT CONNECT ON DATABASE watchbrief TO watchbrief_migrator, watchbrief_api, watchbrief_user_admin;
REVOKE ALL ON SCHEMA public FROM PUBLIC;
ALTER SCHEMA public OWNER TO watchbrief_owner;
\password watchbrief_migrator
\password watchbrief_api
\password watchbrief_user_admin
