-- Dedicated fictional-data DB only. Run as bootstrap after migrate.sql.
-- Everything is rolled back. No password hashes or token hashes are selected/output.
\set ON_ERROR_STOP on
BEGIN;
SET LOCAL ROLE watchbrief_owner;
INSERT INTO users (id, login_id, password_hash)
VALUES ('01ARZ3NDEKTSV4RRFFQ69G5FAA', 'constraint_probe', repeat('x', 90));
DO $$
BEGIN
 BEGIN
  INSERT INTO users VALUES ('invalid', 'invalid_id', repeat('x',90), now(), now());
  RAISE EXCEPTION 'invalid ULID accepted';
 EXCEPTION WHEN check_violation THEN NULL; END;
 BEGIN
  INSERT INTO users VALUES ('01ARZ3NDEKTSV4RRFFQ69G5FAB', 'UPPERCASE', repeat('x',90), now(), now());
  RAISE EXCEPTION 'uppercase login ID accepted';
 EXCEPTION WHEN check_violation THEN NULL; END;
 BEGIN
  INSERT INTO users VALUES ('01ARZ3NDEKTSV4RRFFQ69G5FAB', repeat('a',65), repeat('x',90), now(), now());
  RAISE EXCEPTION 'overlong login ID accepted';
 EXCEPTION WHEN check_violation THEN NULL; END;
 BEGIN
  INSERT INTO users VALUES ('01ARZ3NDEKTSV4RRFFQ69G5FAB', 'constraint_probe', repeat('x',90), now(), now());
  RAISE EXCEPTION 'duplicate login accepted';
 EXCEPTION WHEN unique_violation THEN NULL; END;
 BEGIN
  INSERT INTO auth_sessions (id,user_id,token_hash)
  VALUES ('01ARZ3NDEKTSV4RRFFQ69G5FAC','01ARZ3NDEKTSV4RRFFQ69G5FAB',decode(repeat('aa',32),'hex'));
  RAISE EXCEPTION 'missing user FK accepted';
 EXCEPTION WHEN foreign_key_violation THEN NULL; END;
 BEGIN
  INSERT INTO auth_sessions (id,user_id,token_hash)
  VALUES ('01ARZ3NDEKTSV4RRFFQ69G5FAC','01ARZ3NDEKTSV4RRFFQ69G5FAA',decode('aa','hex'));
  RAISE EXCEPTION 'short token hash accepted';
 EXCEPTION WHEN check_violation THEN NULL; END;
END $$;
INSERT INTO auth_sessions (id,user_id,token_hash)
VALUES ('01ARZ3NDEKTSV4RRFFQ69G5FAC','01ARZ3NDEKTSV4RRFFQ69G5FAA',decode(repeat('aa',32),'hex'));
DO $$ BEGIN
 BEGIN
  INSERT INTO auth_sessions (id,user_id,token_hash)
  VALUES ('01ARZ3NDEKTSV4RRFFQ69G5FAD','01ARZ3NDEKTSV4RRFFQ69G5FAA',decode(repeat('aa',32),'hex'));
  RAISE EXCEPTION 'duplicate token hash accepted';
 EXCEPTION WHEN unique_violation THEN NULL; END;
 BEGIN
  DELETE FROM users WHERE login_id='constraint_probe';
  RAISE EXCEPTION 'referenced user deletion accepted';
 EXCEPTION WHEN foreign_key_violation THEN NULL; END;
END $$;
RESET ROLE;
SET LOCAL ROLE watchbrief_api;
SELECT count(*) AS fictional_user_visible FROM users WHERE login_id='constraint_probe';
DO $$ BEGIN
 BEGIN EXECUTE 'CREATE TABLE forbidden_probe(id int)'; RAISE EXCEPTION 'DDL accepted';
 EXCEPTION WHEN insufficient_privilege THEN NULL; END;
 BEGIN EXECUTE 'ALTER TABLE users ADD COLUMN forbidden_probe int'; RAISE EXCEPTION 'ALTER accepted';
 EXCEPTION WHEN insufficient_privilege THEN NULL; END;
 BEGIN EXECUTE 'CREATE ROLE forbidden_probe'; RAISE EXCEPTION 'role management accepted';
 EXCEPTION WHEN insufficient_privilege THEN NULL; END;
 IF pg_has_role(current_user, 'watchbrief_owner', 'SET') THEN
  RAISE EXCEPTION 'API has owner role membership';
 END IF;
 BEGIN EXECUTE 'CREATE SCHEMA forbidden_probe'; RAISE EXCEPTION 'schema DDL accepted';
 EXCEPTION WHEN insufficient_privilege THEN NULL; END;
 BEGIN EXECUTE 'CREATE TEMP TABLE forbidden_probe(id int)'; RAISE EXCEPTION 'temporary DDL accepted';
 EXCEPTION WHEN insufficient_privilege THEN NULL; END;
 BEGIN UPDATE users SET login_id='forbidden_probe' WHERE login_id='constraint_probe'; RAISE EXCEPTION 'user update accepted';
 EXCEPTION WHEN insufficient_privilege THEN NULL; END;
 BEGIN INSERT INTO users (id,login_id,password_hash) VALUES ('01ARZ3NDEKTSV4RRFFQ69G5FAE','forbidden_probe',repeat('x',90)); RAISE EXCEPTION 'user creation accepted';
 EXCEPTION WHEN insufficient_privilege THEN NULL; END;
 BEGIN DELETE FROM auth_sessions; RAISE EXCEPTION 'session delete accepted before #54';
 EXCEPTION WHEN insufficient_privilege THEN NULL; END;
END $$;
RESET ROLE;
SET LOCAL ROLE watchbrief_user_admin;
INSERT INTO users (id,login_id,password_hash) VALUES ('01ARZ3NDEKTSV4RRFFQ69G5FAE','admin_probe',repeat('x',90));
DO $$ BEGIN
 BEGIN PERFORM * FROM users; RAISE EXCEPTION 'admin read accepted';
 EXCEPTION WHEN insufficient_privilege THEN NULL; END;
 BEGIN EXECUTE 'CREATE TABLE forbidden_probe(id int)'; RAISE EXCEPTION 'admin DDL accepted';
 EXCEPTION WHEN insufficient_privilege THEN NULL; END;
END $$;
ROLLBACK;
\echo 'Constraint and role checks passed (rolled back).'
