# MIT Distributed Systems Labs

## 5 Labs Included

### **Lab 1 - MapReduce (mr)**

   * Fault-tolerant batch processing via a **Coordinator/Worker** design.
   * Task leasing, retries on timeout, idempotent output.
   * Deterministic intermediate file naming and cleanup.

### **Lab 2 - Raft (raft)**

   * Leader election, log replication, commitment, and **persistence** (term, vote, log).
   * Safety (log-matching, election restriction) + liveness with randomized timeouts.
   * **Snapshotting** and `InstallSnapshot` to keep state bounded.

### **Lab 3 - KV over Raft (kvraft)**

   * Linearizable `Get/Put/Append` API backed by Raft.
   * **Exactly-once semantics** via `(ClientID, SeqNo)` deduplication.
   * Apply-loop backpressure and snapshot triggers at byte thresholds.

### **Lab 4 - Sharded KV (shardkv + shardctrler)**

   * **Shard Controller** (separate Raft group) issues configurations.
   * Online **reconfiguration + shard migration** with barrier epochs.
   * Transfers include data **and client dedup state** to preserve linearizability.

### **Lab 5 - Legacy K/V server — kvsrv**

   * Minimal K/V used early on to exercise RPC patterns and timeouts.

---

## Tech & Practices

* **Language:** Go
* **Concurrency:** goroutines, channels, timers; careful lock granularity
* **RPC:** Go `net/rpc`/gob (course harness)
* **Durability:** snapshot compaction; restore-before-serve
* **Testing:** deterministic harness with partitions, reordering, and crashes; `-race` clean

---

## Repo Layout

```
.
├─ mr/               # MapReduce (coordinator, worker, job flow)
├─ raft/             # Raft library (persists term/vote/log; snapshots)
├─ kvsrv/            # Simple KV server (early lab)
├─ kvraft/           # Raft-backed KV service (client/server)
├─ shardctrler/      # Shard config manager (its own Raft group)
├─ shardkv/          # Sharded KV (migration + reconfig)
└─ scripts/          # Helpers for local runs (optional)
```

---

## Design Highlights (What I Optimized For)

* **Correctness first:** all state transitions covered by tests; avoided sleep-based hacks.
* **Failure semantics:** idempotent ops, dedup tables, retryable commits.
* **Throughput without sacrificing safety:** batching where safe, bounded apply queues.
* **Reconfiguration safety:** do not serve a shard until **data + dedup state** is in place and the epoch matches the controller config.
* **Observability:** concise logs keyed by `(term, index, role)`; optional log gating for tests.

---

## Running Tests

> Requires a recent Go toolchain. From repo root:

```bash
# MapReduce
go test -race ./mr

# Raft core (2A–2D tests vary by year naming)
go test -race -run 2A ./raft
go test -race -run 2B ./raft
go test -race -run 2C ./raft
go test -race -run 2D ./raft

# KV over Raft
go test -race -run 3A ./kvraft
go test -race -run 3B ./kvraft

# Sharded KV
go test -race -run 4A ./shardkv
go test -race -run 4B ./shardkv
```

---

## Notes

* The **Raft** package is self-contained and reused by higher layers.
* **Client API** is linearizable; clients discover leaders and retry with index/term acks.
* **Snapshots** are produced on size thresholds and installed out-of-band.
* **Shard migration** is transactional: transfer → install → GC, with epoch checks to prevent split-brain.

---

## References (Course Specs)

* MapReduce: [https://pdos.csail.mit.edu/6.824/labs/lab-mr.html](https://pdos.csail.mit.edu/6.824/labs/lab-mr.html)
* K/V Server: [https://pdos.csail.mit.edu/6.824/labs/lab-kvsrv1.html](https://pdos.csail.mit.edu/6.824/labs/lab-kvsrv1.html)
* Raft: [https://pdos.csail.mit.edu/6.824/labs/lab-raft1.html](https://pdos.csail.mit.edu/6.824/labs/lab-raft1.html)
* KV over Raft: [https://pdos.csail.mit.edu/6.824/labs/lab-kvraft1.html](https://pdos.csail.mit.edu/6.824/labs/lab-kvraft1.html)
* Sharded KV: [https://pdos.csail.mit.edu/6.824/labs/lab-shard1.html](https://pdos.csail.mit.edu/6.824/labs/lab-shard1.html)
