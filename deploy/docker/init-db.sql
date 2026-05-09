-- Docker启动时自动执行的初始化SQL
-- 注意：此文件仅在首次启动容器时执行

\i /docker-entrypoint-initdb.d/migrations/001_init_schema.sql
\i /docker-entrypoint-initdb.d/migrations/002_platform_tables.sql
\i /docker-entrypoint-initdb.d/migrations/003_seed_data.sql
