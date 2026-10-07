-- Test fixtures for the CI throwaway database, loaded after gopay.sql.
-- Tests that write callbacks, logs or tenant configs need these tenants to satisfy the foreign keys.
-- Never run against a real database. Nothing to roll back: the CI database is discarded after the job.
INSERT INTO tenants (id, username, password) VALUES
    (1, 'ci-admin', 'not-a-login'),
    (2, 'ci-tenant', 'not-a-login');

SELECT setval('tenants_id_seq', 2);
