-- Docker 容器首次启动时自动执行
-- 执行顺序：核心表 → 中台表 → RLS 策略 → 族谱表 → 种子数据
\i /docker-entrypoint-initdb.d/migrations/001_init_schema.sql
\i /docker-entrypoint-initdb.d/migrations/002_platform_tables.sql
\i /docker-entrypoint-initdb.d/migrations/004_rls_policies.sql
\i /docker-entrypoint-initdb.d/migrations/005_genealogy_tables.sql
\i /docker-entrypoint-initdb.d/migrations/003_seed_data.sql
