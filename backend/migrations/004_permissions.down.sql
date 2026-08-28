DROP INDEX IF EXISTS idx_user_permissions_user;
DROP INDEX IF EXISTS uq_user_permissions_global;
DROP INDEX IF EXISTS uq_user_permissions_project;
DROP TABLE IF EXISTS user_permissions;
ALTER TABLE users ALTER COLUMN role DROP DEFAULT;
