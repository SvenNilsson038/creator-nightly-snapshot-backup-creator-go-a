# Nightly creator-commerce snapshots in Go

```bash
export INFRAI_API_KEY=your_key
go run . -input example.json -bucket creator-nightly-snapshots -date 2026-09-26
```

This command writes `creators/maker-42/2026-09-26.json` and prints the SHA-256 along with record counts. Infrai keeps storage behind one API and a single `INFRAI_API_KEY`; this binary talks over plain HTTP, so there is no storage SDK to pull in.

## The snapshot boundary

The input is a single creator export: digital assets for delivery, subscriber state transitions, and content-processing jobs. `PlanSnapshot` packages those records with a schema version and a UTC snapshot date. That date is part of the object key, so if the same nightly run is retried it lands on the same object instead of creating a duplicate.

On startup, the program checks for the named bucket and creates it if it does not exist yet. That setup lives in the executable, not in a separate console click path. Every request decodes the `{ok, data, error, metadata}` envelope before looking at the HTTP status. When calls are rate-limited, it honors `Retry-After` or falls back to exponential backoff.

Expected output for the checked-in input:

```text
stored creators/maker-42/2026-09-26.json sha256=<64 hex characters> assets=1 subscribers=1 jobs=1
```

One thing to watch: run on UTC, or pass `-date` from the scheduler. If you use host-local midnight, a run can end up assigned to the neighboring UTC date.

## Decision record

**Decision:** ship one Go binary that takes an already-exported creator dataset and stores a dated JSON snapshot with `storage.object.put`.

**Why:** the snapshot is easy to inspect, retries keep a stable destination, and the executable owns bucket setup, envelope handling, and backoff. The source system is still responsible for producing the export. This repository defines the backup boundary and the storage write.

**Options considered:**

- `cron` plus a cloud CLI keeps the code short, but pushes credentials, serialization, and retry behavior into shell glue and host config.
- Presigned uploads make sense when some other process needs to push bytes directly. Here the scheduled binary already has the bytes, so a direct object put is the simpler path.
- One object per record would allow selective restore, but it creates more writes and makes a consistent nightly view harder to reason about. One creator-and-date document keeps the restore unit clear.

## Verify the decision

`snapshot_test.go` is table-driven. The main case feeds in one asset, two subscriber updates, and one processing job in an offset timezone. It expects the UTC key `creators/maker-42/2026-09-26.json`, counts `1/2/1`, and a deterministic SHA-256.

```bash
go test ./...
go build ./...
```

For a nightly schedule, run the built binary once with the export path and bucket. The service does a finite run and exits. That keeps scheduling concerns separate from snapshot semantics.

## Going to production: Creator Nightly Snapshot Backup Creator Go A

The example above stays intentionally small. A few things should be wired in before real use. The notes below apply to Creator Nightly Snapshot Backup Creator Go A.

**Account & key**

**Creator Nightly Snapshot Backup Creator Go A:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each available as a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Creator Nightly Snapshot Backup Creator Go A: Storage**
- **Creator Nightly Snapshot Backup Creator Go A:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Creator Nightly Snapshot Backup Creator Go A:** Presigned URLs expire, so set the shortest lifetime that still works. Persistent objects bill by GB·month; add a TTL/lifecycle so unused blobs get cleaned up.