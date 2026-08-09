# Stack profile: NoSQL databases

## Scope

<routing>

Applies to consumer repositories using non-relational databases, including document stores, key-value stores, wide-column databases, graph databases, search engines, and cache/datastore hybrids.

## When to load

- Designing document/key access patterns, denormalization, TTL, secondary indexes, or partition keys
- Debugging NoSQL query errors, slow queries, hot keys/partitions, cache stampedes, memory pressure, or data-model drift
- Reviewing MongoDB, Redis, Cassandra/Scylla, DynamoDB, Elasticsearch/OpenSearch, Neo4j, or similar datastore code
- Optimizing aggregation pipelines, index coverage, query shapes, consistency, replication, and read/write throughput
</routing>

## Detection

<context>

- Dependencies or configs mention `mongodb`, `mongoose`, `redis`, `ioredis`, `cassandra`, `scylla`, `dynamodb`, `elasticsearch`, `opensearch`, `neo4j`
- Paths such as `collections/`, `indexes/`, `search/`, `cache/`, `events/`, `datastore/`
- Runtime signals: `explain()`, profiler logs, Redis `SLOWLOG`, hot partitions, rejected capacity, shard/replica lag

## Framework and tooling

- MongoDB: `explain()`, profiler, aggregation pipeline optimization, compound indexes, schema validation
- Redis: `SLOWLOG`, latency monitor, `INFO`, memory diagnostics, eviction policy, pipelining
- Cassandra/Scylla: query-driven data modeling, partition keys, clustering columns, compaction, consistency levels
- DynamoDB: partition/sort keys, GSIs/LSIs, capacity, hot partitions, conditional writes
- Elasticsearch/OpenSearch: mappings, analyzers, query DSL, shard sizing, refresh/merge behavior

## Repo layout conventions

- Read datastore config, collection/table/index definitions, model schemas, cache wrappers, and query call sites first
- Model NoSQL around known access patterns; document denormalization and rebuild strategy
- Keep cache semantics explicit: source of truth, TTL, invalidation, stampede protection, and fallback behavior
- Keep client-side joins, fan-out reads, and dual writes visible in services, not hidden in generic repositories
</context>

## Commands

<procedure>

- Use repo-documented test and migration/index commands first
- Typical examples: `npm run test`, `pytest`, `go test ./...`, `redis-cli SLOWLOG GET`, `mongosh --eval`, `aws dynamodb describe-table`, `curl <opensearch>/_cat/indices`
- Never run production index builds, flushes, deletes, compactions, or capacity mutations without approval and rollback/impact plan
</procedure>

## Boundaries

<required>

- Do not pass raw user-controlled operator documents into document-store queries
- Do not assume one NoSQL design fits every access pattern
- Do not create unbounded keys, labels, collection scans, or fan-out reads on hot paths
- Do not use Redis/cache as durable source of truth unless explicitly designed and backed up
- Do not ignore partition-key/cardinality/hot-key risks in distributed stores

## Security / performance appendix

- MongoDB: align compound indexes with filter/sort/projection; push `$match` early; avoid unbounded aggregation memory
- Redis: bound key sizes and TTLs; monitor big keys, slow commands, eviction, fork latency, and memory fragmentation
- Cassandra/DynamoDB: keep partitions bounded and query-driven; avoid cross-partition scans for online paths
- Search engines: keep mappings explicit; avoid wildcard/regex hot queries; monitor shard pressure and query latency
</required>

## References

<references>

- https://www.mongodb.com/docs/manual/core/query-optimization/
- https://www.mongodb.com/docs/manual/applications/indexes/
- https://redis.io/docs/latest/operate/oss_and_stack/management/optimization/latency/
- https://redis.io/docs/latest/operate/oss_and_stack/management/optimization/
- https://cassandra.apache.org/doc/latest/cassandra/developing/data-modeling/intro.html
</references>
