-- Create temporal user for Temporal server
DO
$do$
BEGIN
   IF NOT EXISTS (
      SELECT FROM pg_catalog.pg_roles
      WHERE  rolname = 'temporal') THEN

      CREATE ROLE temporal LOGIN SUPERUSER CREATEDB CREATEROLE PASSWORD 'temporal';
   END IF;
END
$do$;

-- Create temporal database for Temporal server
SELECT 'CREATE DATABASE temporal'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'temporal')\gexec
