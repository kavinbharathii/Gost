# Gost

A persistent, replicated key-value store written in Go from scratch. No external dependencies. Built to understand how systems like Redis work under the hood — storage engine, WAL, TTL, replication, and WAL compaction.

---

## Architecture

```
client (telnet / gost-client)
        │
        ▼
┌───────────────────┐
│     TCP Server    │  one goroutine per connection
│   (server/)       │
└────────┬──────────┘
         │
         ▼
┌───────────────────┐        ┌─────────────────────┐
│      Store        │──WAL──▶│   gost-{port}.wal   │
│   (store/)        │        └─────────────────────┘
│                   │
│  sync.RWMutex     │
│  map[string]string│  in-memory
│  map[string]Time  │  expiry index
└───────────────────┘
         │
         ▼ (leader mode only)
┌───────────────────┐
│  Replication      │  streams WAL entries to followers
│  (replication/)   │  over a dedicated TCP port
└───────────────────┘
```

### Packages

- `store/` — core KV store. `sync.RWMutex` over a `map[string]string` with a parallel `map[string]time.Time` for TTL tracking. All writes go to the WAL before the in-memory map.
- `protocol/` — line-based text protocol parser. Parses raw TCP input into typed `Command` structs.
- `server/` — TCP listener. One goroutine per client via `go handleConn(conn)`. Enforces leader/follower write rules.
- `replication/` — leader streams WAL entries to followers over a dedicated port. Followers connect, send their current WAL offset, receive catch-up entries, then stay connected for real-time streaming.

---

## Storage

### Write-Ahead Log (WAL)

Every `SET` and `DEL` is appended to `gost-{port}.wal` before the in-memory map is updated. On startup, Gost replays the WAL to reconstruct state.

WAL format:
```
SET name kavin
SET session abc EX 1748251234
DEL name
```

TTL values are stored as absolute Unix timestamps, not relative durations — so expiry is preserved correctly across restarts.

### WAL Compaction

The WAL grows unbounded over time. `COMPACT` rewrites it to only the latest state of each key, dropping deleted keys, overwritten values, and expired TTLs.

Compaction uses a lock-minimizing two-phase approach:
1. Record the current WAL line offset. Replay the WAL up to that offset into a temp map. Write the temp map to `gost-{port}.wal.compact` — all outside any lock.
2. Acquire a full write lock. Append only the new WAL entries written since the offset to the compact file. Atomically rename `gost-{port}.wal.compact` → `gost-{port}.wal`. Reopen the WAL file handle. Release lock.

The write lock only covers step 2 — appending a few lines and a rename. Microseconds, not seconds.

---

## TTL

Keys support expiry via the `EX` flag:

```
SET session abc EX 3600
```

Two expiry mechanisms run concurrently:

- **Lazy expiry** — on every `GET`, if the key has expired, it is deleted and `$-1` is returned. No client-visible difference from a missing key.
- **Active expiry** — a background goroutine sweeps the expiry index every 5 seconds and deletes stale keys. Prevents unbounded memory growth from keys nobody reads.

---

## Replication

Gost supports leader/follower replication. The leader accepts reads and writes. Followers accept reads only and reject writes with `-ERR this is a follower node, writes not allowed`.

Each instance has its own independent WAL (`gost-{port}.wal`) and in-memory store.

**How it works:**

1. Follower connects to the leader's replication port and sends its current WAL line count as an offset.
2. Leader replays its WAL from that offset, streaming catch-up entries to the follower over TCP.
3. Leader registers the follower for real-time updates. Every subsequent `SET`/`DEL` is published to all connected followers via a buffered channel per follower.
4. If a follower is too slow to consume its channel, entries are dropped rather than blocking the leader.
5. On reconnect, the follower re-sends its offset and the process repeats — it catches up on everything it missed.

---

## Protocol

Line-delimited text protocol over TCP. Human-readable and testable with `telnet`.

| Command | Request | Response |
|---|---|---|
| Set a key | `SET key value` | `+OK` |
| Set with TTL | `SET key value EX <seconds>` | `+OK` |
| Get a key | `GET key` | `+value` or `$-1` |
| Delete a key | `DEL key` | `+OK` or `$-1` |
| Compact WAL | `COMPACT` | `+OK` |

Commands are case-insensitive. `$-1` indicates a nil/missing key.

---

## Running

```bash
# leader
go run main.go --mode leader --port 6379 --repl-port 6380

# follower
go run main.go --mode follower --port 6381 --leader localhost:6380

# connect
telnet localhost 6379
```

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--mode` | `leader` | `leader` or `follower` |
| `--port` | `6379` | client listener port |
| `--repl-port` | `6380` | replication listener port (leader only) |
| `--leader` | `""` | leader replication address (follower only) |

---

## Roadmap

- [ ] Go client library
- [ ] Raft consensus for automatic leader election
- [ ] `KEYS` and `EXISTS` commands
- [ ] Per-instance config file
- [ ] Prometheus metrics endpoint
