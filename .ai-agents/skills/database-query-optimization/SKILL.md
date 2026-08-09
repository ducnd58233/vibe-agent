---
name: database-query-optimization
description: >-
  Diagnoses, optimizes, and hardens SQL and NoSQL data access: query errors, slow queries, indexes, explain plans, N+1 patterns, migrations, transactions, locks, hot keys/partitions, cache behavior, and data-model/query-shape alignment. Use when working on SQL/PostgreSQL/MySQL/SQLite/ORM queries or NoSQL/MongoDB/Redis/Cassandra/DynamoDB/Search datastore issues.
disable-model-invocation: true
---

# Database Query Optimization

## How

<procedure>

1. **Load stack context**
   - Inspect manifests, migrations, schema files, ORM config, repository modules, and query call sites.
   - Open [`stack-profiles/ROUTER.md`](../../stack-profiles/ROUTER.md); load [`sql-databases.md`](../../stack-profiles/sql-databases.md) and/or [`nosql-databases.md`](../../stack-profiles/nosql-databases.md) plus backend/runtime profiles.
   - Use [`references/database-query-patterns.md`](../../references/database-query-patterns.md).
2. **Capture the failure or objective**
   - Error text, slow query symptom, timeout, deadlock, lock wait, memory spike, hot partition/key, stale cache, or correctness mismatch.
   - Preserve the exact query/command and bound parameters where safe.
3. **Identify query shape**
   - Filters, sort, projection, joins/lookups, grouping, pagination, write path, consistency, expected cardinality, and frequency.
4. **Collect evidence**
   - SQL: `EXPLAIN`/`EXPLAIN ANALYZE`, indexes, constraints, row counts, locks, generated ORM SQL.
   - NoSQL: `explain()`, profiler/slow log, index definitions, key sizes, partition/cardinality, capacity/latency metrics.
5. **Choose the smallest fix**
   - Query rewrite, index change, data-model change, transaction boundary fix, batching, keyset pagination, cache policy, denormalization, or async/reporting path.
   - Include write-amplification, migration, rollback, and operational risk.
6. **Verify**
   - Compare before/after plan, timing, cardinality, and error behavior.
   - Add regression tests for query errors and monitoring/slow-log checks for hot paths.
</procedure>

## Routing & discovery

<routing>

- Pair with [`backend-engineering`](../backend-engineering/SKILL.md) for repository/transaction boundaries.
- Pair with [`security-and-hardening`](../security-and-hardening/SKILL.md) for injection, raw operators, and least-privilege roles.
- Pair with [`performance-optimization`](../performance-optimization/SKILL.md) for broader service latency work.
- Pair with [`observability-monitoring`](../observability-monitoring/SKILL.md) for slow-query dashboards and alerts.

Use for database query analysis, optimization, query error detection, schema/index review, ORM-generated query review, and cache/datastore performance issues. Do not use for general backend layering without a database concern; use [`backend-engineering`](../backend-engineering/SKILL.md) instead.
</routing>

## Permissions & authority

<required>

- Tools: Read, Grep, Glob, Edit; Shell only for repo-documented tests, local explain/profile commands, and safe validation.
- Paths: source, tests, migrations, schema, query fixtures, docs; no secrets.
- Ask before production database access, migrations, destructive SQL/commands, index builds, cache flushes, or capacity changes.
</required>

## Verification

<verification>

- [ ] Exact query/command and safe parameters captured.
- [ ] Schema/index/data-model context inspected.
- [ ] Explain/profile/slow-log evidence supports the diagnosis.
- [ ] Fix is scoped and includes migration/rollback impact where relevant.
- [ ] Tests or monitoring guard the regression.
</verification>

## References

<references>

- https://www.postgresql.org/docs/current/using-explain.html
- https://www.postgresql.org/docs/current/indexes-examine.html
- https://www.mongodb.com/docs/manual/core/query-optimization/
- https://redis.io/docs/latest/operate/oss_and_stack/management/optimization/latency/
- https://cassandra.apache.org/doc/latest/cassandra/developing/data-modeling/intro.html
</references>
