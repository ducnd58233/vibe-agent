# Database query analysis patterns

Use this reference for SQL and NoSQL query correctness, performance, and error diagnosis.

## Universal workflow

1. **Identify the query shape**
   - Inputs, filters, sort, projection, joins/lookup, pagination, aggregation, write path, and expected row/document count.
2. **Collect evidence**
   - Error text, generated query, parameters, schema/index definitions, explain plan/profile, timings, cardinality, and workload frequency.
3. **Classify the failure**
   - Syntax/type error, missing relation/index, migration drift, permission, lock/contention, timeout, memory spill, full scan, hot partition/key, or consistency issue.
4. **Optimize the bottleneck**
   - Change query shape, add/adjust index, fix data model, batch, paginate, cache, denormalize, or move to async/reporting path.
5. **Verify and guard**
   - Compare before/after plans and timings; add tests, migration checks, or monitoring.

## SQL red flags

- Unparameterized SQL or string-built filters
- N+1 queries hidden behind ORM relations
- Missing or unused indexes on hot filters/sorts/joins
- Offset pagination on large result sets
- Long transactions holding locks across I/O
- Cardinality estimates far from actual rows
- Sorting/hashing spilling to disk
- Schema drift between migrations and generated models

## NoSQL red flags

- MongoDB collection scans on hot paths
- Raw user-controlled operators in query documents
- Aggregation pipelines that filter late or sort before reducing candidates
- Redis big keys, unbounded TTL-less keys, blocking commands, cache stampedes
- Cassandra/DynamoDB hot partitions or online cross-partition scans
- Search wildcard/regex queries over large indexes
- Dual writes without reconciliation or rebuild plan

## Evidence checklist

- [ ] Query and bound parameters are known.
- [ ] Schema/index/model definitions are known.
- [ ] Explain/profile/slow-log evidence exists for performance claims.
- [ ] Workload frequency and data volume are estimated.
- [ ] Correctness and security risks are separated from performance risks.
- [ ] Recommended fix includes migration/rollback/test implications.

## References

- https://www.postgresql.org/docs/current/using-explain.html
- https://www.postgresql.org/docs/current/indexes-examine.html
- https://www.mongodb.com/docs/manual/core/query-optimization/
- https://redis.io/docs/latest/operate/oss_and_stack/management/optimization/latency/
- https://cassandra.apache.org/doc/latest/cassandra/developing/data-modeling/intro.html
