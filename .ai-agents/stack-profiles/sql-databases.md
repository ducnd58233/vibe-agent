# Stack profile: SQL databases

## Scope

Applies to consumer repositories using relational databases and SQL query engines, including PostgreSQL, MySQL/MariaDB, SQLite, SQL Server, CockroachDB, and cloud SQL variants.

## When to load

- Designing schemas, migrations, indexes, constraints, or transactions
- Debugging SQL errors, deadlocks, lock waits, slow queries, missing indexes, or N+1 patterns
- Reviewing repository/ORM/query-builder code, raw SQL, database migrations, or connection-pool settings
- Optimizing query plans, pagination, joins, aggregation, materialized views, and reporting queries

## Detection

- Manifests or code mention `postgres`, `postgresql`, `mysql`, `mariadb`, `sqlite`, `sqlserver`, `cockroach`
- ORM/query tools such as Prisma, Drizzle, TypeORM, Sequelize, SQLAlchemy, Alembic, Django ORM, ActiveRecord, Diesel, SQLx, jOOQ, Hibernate, Knex
- Files/paths such as `migrations/`, `schema.sql`, `prisma/schema.prisma`, `alembic.ini`, `db/`, `sql/`, `*.sql`

## Framework and tooling

- Use the repo's database engine and ORM/query builder first
- PostgreSQL: `EXPLAIN`, `EXPLAIN ANALYZE`, `pg_stat_statements`, indexes, constraints, vacuum/analyze, locks
- MySQL/MariaDB: `EXPLAIN`, slow query log, indexes, InnoDB locks, query plans
- SQLite: `EXPLAIN QUERY PLAN`, indexes, WAL mode, transaction boundaries
- Test tools: ephemeral DBs, migrations in test setup, fixtures/factories, testcontainers where configured

## Repo layout conventions

- Read manifests, migrations, schema definitions, ORM config, generated types, and repository modules before editing
- Keep migrations reversible where policy requires it; document irreversible data migrations
- Keep transaction orchestration in service/use-case layers; repositories accept existing transaction handles when needed
- Keep external DTOs separate from persistence rows/entities
- Add or update indexes with the query pattern, selectivity, write cost, and migration risk in mind

## Commands

- Use repo-documented migration/test commands first
- Typical examples: `npm run test`, `pytest`, `cargo test`, `go test ./...`, `prisma migrate diff`, `alembic upgrade head --sql`, `sqlx prepare`, `diesel migration run`
- Never run production migrations or destructive SQL without explicit approval and backup/rollback plan

## Boundaries

- Do not concatenate untrusted input into SQL
- Do not add indexes blindly without workload evidence or expected query shape
- Do not hide multi-table business transactions inside a single repository
- Do not use offset pagination for high-page-number hot paths without checking keyset/seek pagination alternatives
- Do not treat ORM-generated SQL as safe from performance review

## Security / performance appendix

- Inspect plan shape: sequential scans, join order, cardinality estimates, sort/hash spills, index usage, row counts, and buffers where available
- Track locks, long transactions, connection pool saturation, replication lag, dead tuples/bloat, and slow query percentiles
- Prefer constraints for invariants the database can enforce
- Use parameterized queries and least-privilege database roles

## References

- https://www.postgresql.org/docs/current/using-explain.html
- https://www.postgresql.org/docs/current/indexes-examine.html
- https://dev.mysql.com/doc/refman/en/using-explain.html
- https://www.sqlite.org/eqp.html
- https://owasp.org/www-community/attacks/SQL_Injection
