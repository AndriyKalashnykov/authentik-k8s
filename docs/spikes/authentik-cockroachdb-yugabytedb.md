# Spike: Authentik on CockroachDB or YugabyteDB instead of PostgreSQL

> **Question.** Earlier attempts to run Authentik on **CockroachDB** or **YugabyteDB** (instead of
> PostgreSQL) failed — the documented blocker being that Authentik's Django migrations require
> PostgreSQL advisory locks (`pg_advisory_lock()`), which CockroachDB does not implement. *Are
> those issues fixed now (mid-2026)?*
>
> **Scope.** Authentik is pinned to **2026.5** in this repo; the production-working datastore is
> PostgreSQL (`postgres:18-alpine`). An experimental, known-broken `k8s/cockroachdb/` backend
> still lives in the tree (frozen — Renovate ignores `k8s/cockroachdb/**`).
>
> **Evidence date:** 2026-06-26. All claims below cite primary sources (see [References](#8-references)).

## 1. Summary

| Database | Verdict on Authentik 2026.5 |
|----------|------------------------------|
| **CockroachDB** | **Still blocked.** Session-scoped `pg_advisory_lock()` — the exact function Authentik's migrations need — is **not yet implemented**. CockroachDB added *transaction-level* advisory locks in early 2026, but the *session-scoped* builtins are tracked by an issue opened **2026-05-08** that is still **open with no milestone** ([cockroachdb#169981](https://github.com/cockroachdb/cockroach/issues/169981)). Authentik also has no flag to disable advisory locks ([authentik#14562](https://github.com/goauthentik/authentik/issues/14562), open, unanswered). |
| **YugabyteDB** | **Original blocker FIXED, but Authentik still does not install — VALIDATED on KinD (see §6a).** Advisory locks are GA + default-on since v2025.1 and YSQL is PostgreSQL 15 (in range), so the *advisory-lock* blocker the user hit is genuinely gone (`pg_try_advisory_lock()` → true; Authentik's migration lock acquires/releases). **But** a POC on YugabyteDB 2025.1.4.0 + Authentik 2026.5 showed the Django migrations **abort partway** with YugabyteDB transaction-conflict errors (`YB001 "transaction expired or aborted"`), **even with `yb_enable_read_committed_isolation=true`**. So: the *old* issue is fixed, a *different*, deeper DDL/migration-transaction incompatibility now blocks a clean install. |

Bottom line: **CockroachDB is still a no.** **YugabyteDB is the one that changed** — the specific
issue that broke it before is fixed upstream, making a fresh POC attempt reasonable (but it must be
proven, not assumed).

## 2. Authentik's database requirements

Authentik **requires PostgreSQL** and treats it as the exclusive backend — there is no supported
non-PostgreSQL story.

| Requirement | Detail | Source |
|-------------|--------|--------|
| Engine | "authentik requires PostgreSQL for application data, configuration, sessions, and background task coordination." No alternative database is documented. | [Configuration docs](https://docs.goauthentik.io/install-config/configuration/) |
| Version range | **PostgreSQL 14 – 18.** Minimum 14 since release 2024.6 ("authentik now requires PostgreSQL version 14 or later"); the bundled image moved 15→17 in 2025.6. | [Configuration docs](https://docs.goauthentik.io/install-config/configuration/), [Release 2024.6](https://docs.goauthentik.io/releases/2024.6/) |
| Advisory locks (migrations) | Authentik's Django migrations take a PostgreSQL **advisory lock** to serialize concurrent server/worker startup migrations. CockroachDB's missing `pg_advisory_lock()` is the documented failure (this repo's `CLAUDE.md` / `README.md`). | this repo; [authentik#14562](https://github.com/goauthentik/authentik/issues/14562) |
| Advisory locks (runtime sync) | Since **2024.6**, Authentik also uses "postgres advisory locks instead of redis locks" for **SCIM providers** and **LDAP sources** sync. So advisory locks are now load-bearing at *runtime*, not just at migration time. | [Release 2024.6](https://docs.goauthentik.io/releases/2024.6/) |

### Why advisory locks matter here

The failure is specifically about **session-scoped** advisory locks. Authentik's migration lock and
its sync locks are acquired with `pg_advisory_lock()` / `pg_try_advisory_lock()` (held until released
or the session ends) rather than the transaction-scoped `pg_advisory_xact_lock()` (auto-released at
commit). A database that implements *only* transaction-scoped locks (CockroachDB's current state) is
**not sufficient** — it must implement the session-scoped builtins.

[authentik#14562](https://github.com/goauthentik/authentik/issues/14562) ("Flag to disable advisory
locks", opened 2025-05-19) enumerates **5 code locations** that use advisory locks — migration, LDAP
sync, Kerberos sync, and outgoing/SCIM sync. As of 2026-06-26 the issue is **open with no maintainer
response** and **no flag exists** to disable them. There is no in-Authentik escape hatch.

## 3. CockroachDB

**Latest stable line:** the v25.x series (CockroachDB ships ~half-yearly; the May-2026 advisory-lock
work targets the future 26.x line). The experimental backend in this repo pins the long-EOL
**`cockroachdb/cockroach:v22.2.19`** (`k8s/cockroachdb/`).

### Advisory-lock status (the blocker)

CockroachDB's advisory-lock support is **partial and still incomplete for Authentik's needs**:

| Lock type | Status | Source |
|-----------|--------|--------|
| Stubbed (lie-about-locking, no-op) | The original behavior — `pg_advisory_lock` was a stub or unknown function. The umbrella issue [cockroachdb#13546](https://github.com/cockroachdb/cockroach/issues/13546) ("fill out pg_advisory_lock stubs", opened **2017-02-12**) is now **Closed**, superseded by real implementation work. Maintainer @bdarnell: *"it seems bad to lie about locking like this."* | [cockroachdb#13546](https://github.com/cockroachdb/cockroach/issues/13546) |
| **Transaction-scoped** (`pg_advisory_xact_lock`) | **Implemented** (PR #168355). | referenced in [cockroachdb#169981](https://github.com/cockroachdb/cockroach/issues/169981) |
| **Session-scoped** (`pg_advisory_lock`, `pg_try_advisory_lock`, `*_shared`, `pg_advisory_unlock*`) | **NOT implemented.** Tracked by [cockroachdb#169981](https://github.com/cockroachdb/cockroach/issues/169981), opened **2026-05-08**, **open, no milestone** (Epic CRDB-63745). A related issue ([#170014](https://github.com/cockroachdb/cockroach/issues/170014), `lock_timeout` support) targets **26.3.0**. | [cockroachdb#169981](https://github.com/cockroachdb/cockroach/issues/169981) |

The function whose absence breaks Authentik — **`pg_advisory_lock()` (session-scoped)** — is exactly
the one still missing. The error this repo documents (`unknown function: pg_advisory_lock()`,
SQLSTATE 42883) remains the expected outcome on current CockroachDB.

### Other blockers beyond advisory locks

Even if session-scoped locks land, CockroachDB has historically diverged from PostgreSQL in ways that
break Django/Authentik (broad PG-catalog-compat gaps, sequence/`SERIAL` semantics, ORM edge cases —
the reason a six-year trail of "support CockroachDB" requests exists across many Django/Go/Ruby
migration tools). Advisory locks are the *first* wall, not necessarily the only one — none of those
have been re-validated against Authentik here.

### Verdict — CockroachDB: **No (still blocked).**

The specific issue that broke it before is **not fixed**. Session-scoped advisory locks are an
**open, unmilestoned** upstream feature request (opened weeks before this spike), and Authentik
offers no flag to avoid them.

## 4. YugabyteDB

YugabyteDB's YSQL is a PostgreSQL **fork** (not just wire-compatible) that reuses the upstream PG
query layer, so it tracks PG features much more closely than CockroachDB.

### Advisory-lock status (the former blocker — now resolved)

| Aspect | Status | Source |
|--------|--------|--------|
| Advisory locks (`pg_advisory_lock`, `pg_try_advisory_lock`, session + transaction level) | **GA and enabled by default since v2025.1.** "Advisory locks are available in v2025.1 or later, and enabled by default." Globally visible across nodes/sessions via a dedicated `pg_advisory_locks` system table. | [Explicit locking docs](https://docs.yugabyte.com/stable/explore/transactions/explicit-locking/) |
| Tracking issue | [yugabyte#3642](https://github.com/yugabyte/yugabyte-db/issues/3642) ("[YSQL] Add support for advisory_locks", opened 2020-02-13) is **Closed / Done**. Enabling flag `ysql_yb_enable_advisory_locks` is on by default. | [yugabyte#3642](https://github.com/yugabyte/yugabyte-db/issues/3642) |

### PostgreSQL-major compatibility (matters for Authentik's 14–18 range)

| YugabyteDB line | YSQL based on | Fits Authentik 14–18? |
|-----------------|---------------|------------------------|
| ≤ v2024.2 (and all pre-v2.25) | **PostgreSQL 11.2** | **No** (below Authentik's minimum 14) |
| **v2025.1 (stable) / v2.25 (preview) and later** | **PostgreSQL 15** | **Yes** (15 is within 14–18) |

The advisory-lock fix and the PG-15 rebase arrived **in the same release** (v2025.1, GA early 2026),
which is convenient: the version that adds the locks is also the first version old enough on the
PG-major axis to satisfy Authentik. Earlier YugabyteDB versions fail Authentik on *both* axes
(PG 11.2 < 14, and no advisory locks).

### Remaining unknowns / other blockers

- **No known Authentik-on-YugabyteDB success report.** This verdict is "the documented blocker is
  gone + version axes line up", not "someone ran it green."
- **DDL / migration caveats.** YugabyteDB has historically had limitations and performance cliffs
  around heavy online DDL and some catalog operations; Authentik's migrations are large and
  numerous. This is the most likely place a real attempt would still stumble and must be tested.
- **`pg_advisory_locks` as a system table** (vs PG's in-memory locks) is a behavioral difference;
  semantics are documented as equivalent, but high-churn sync locking is unproven for Authentik.

### Verdict — YugabyteDB: **Partial (feasible candidate, unvalidated).**

The exact issue that broke it before — missing advisory locks — **is fixed** as of v2025.1, and YSQL
is now PG-15-based (inside 14–18). A POC is reasonable; success is **not guaranteed** until the full
migration + e2e flow is run.

## 5. Verdict table

| DB | Session advisory locks? | PG-major compat (vs Authentik 14–18) | Other blockers | Feasible on Authentik 2026.5? | Evidence date |
|----|--------------------------|--------------------------------------|----------------|-------------------------------|---------------|
| **PostgreSQL** (current) | Native | 18 ✓ (repo uses `postgres:18-alpine`) | None — supported path | **Yes (production)** | 2026-06-26 |
| **CockroachDB** | **No** — only transaction-scoped done; session-scoped [#169981](https://github.com/cockroachdb/cockroach/issues/169981) open, no milestone | n/a (blocked earlier) | Broad PG-catalog/ORM divergence; no Authentik disable flag ([#14562](https://github.com/goauthentik/authentik/issues/14562)) | **No** | 2026-06-26 |
| **YugabyteDB** | **Yes** — GA + default since v2025.1 | **PG 15** ✓ (v2025.1+ only; ≤2024.2 was PG 11.2 ✗) | DDL/migration scale untested for Authentik; no known e2e report | **Partial** (candidate, unproven) | 2026-06-26 |

## 6. POC path / what would need to change

### CockroachDB — what would have to change upstream (don't attempt yet)

There is **no viable workaround today**. A working CockroachDB backend requires *all* of:

1. **Upstream CockroachDB** ships session-scoped `pg_advisory_lock()` etc. ([#169981](https://github.com/cockroachdb/cockroach/issues/169981) — open, no milestone; the related `lock_timeout` piece targets 26.3.0, so realistically a future 26.x).
2. **OR Authentik** adds a flag to disable / swap the lock backend ([#14562](https://github.com/goauthentik/authentik/issues/14562) — open, no maintainer response, no flag).
3. Re-validation of every *other* PG-compat assumption (sequences, catalog queries, Django ORM) once locks exist.

Until at least (1) or (2) lands, the `k8s/cockroachdb/` backend cannot work. No sidecar / shim
provides session-scoped advisory locks externally.

### YugabyteDB — a concrete POC outline grounded in this repo

A YugabyteDB attempt would mirror the existing `k8s/cockroachdb/` structure. Sketch:

1. **New directory** `k8s/yugabytedb/` (do **not** overwrite the frozen CockroachDB manifests).
   Deploy YugabyteDB **v2025.1 or later** (PG-15 / advisory-locks GA) — e.g. the official
   `yugabytedb/yugabyte` image or the YugabyteDB Helm chart rendered with `helm template`, matching
   how `k8s/postgresql/` is generated.
2. **Point Authentik at YSQL** via the same env contract the other backends use:
   `AUTHENTIK_POSTGRESQL__HOST` = the YSQL service, `AUTHENTIK_POSTGRESQL__PORT` = `5433` (YSQL's
   default PG-wire port), plus `__NAME` / `__USER` / `__PASSWORD`. Pre-create the database/role
   (an init job, like the CockroachDB manifests' `db-create` initContainer).
3. **Confirm advisory locks are live** before migrating:
   `SELECT pg_advisory_lock(1); SELECT pg_advisory_unlock(1);` against the YSQL endpoint must
   succeed (proves `ysql_yb_enable_advisory_locks` default-on).
4. **Run the migrations** — the real test. Start `authentik-server` / `authentik-worker` and watch
   the migration job complete (this is where the CockroachDB path dies). Watch for advisory-lock
   errors *and* any DDL/catalog migration failures.
5. **Run the existing verification flow** — `make e2e-compose` or `make e2e` exercise the full
   provision-and-verify path (groups, users, tokens, re-auth). Green e2e against a YugabyteDB
   backend = the POC is proven.
6. **Pin + (optionally) Renovate-track** the YugabyteDB image only if it works. Keep it isolated so
   a partial attempt cannot regress the PostgreSQL path.

Verification gate: the spike is "done = works" only when the migration job completes **and** `make
e2e` passes against YugabyteDB — not merely when advisory locks resolve.

### 6a. POC validation results (executed 2026-06-26 on KinD)

The POC outline above was **built and run** — `k8s/yugabytedb/authentik-yugabytedb.yml` (single-node
`yugabytedb/yugabyte:2025.1.4.0-b103`, YSQL on 5433, `ysql_enable_auth=true`) deployed on KinD with
Authentik 2026.5's server/worker/valkey pointed at it via the `authentik` Secret. Findings, in order:

| Step | Result |
|------|--------|
| YugabyteDB starts, YSQL ready | ✅ `PostgreSQL 15.12-YB-2025.1.4.0` |
| Advisory locks (direct) | ✅ `SELECT pg_try_advisory_lock(42)` → `t`; `pg_advisory_unlock(42)` → `t` |
| Advisory locks (Authentik's migration lock) | ✅ log shows `waiting to acquire database lock` → `releasing database lock` |
| `rowcount` semantics (a suspected divergence) | ✅ identical to PostgreSQL — a 0-row `SELECT` returns `rowcount=0` (ruled out as a cause) |
| **Django migrations complete** | ❌ **abort ~35–40 migrations in** (around `authentik_blueprints.0001_initial`) with `psycopg.errors.SerializationFailure` / `OperationalError: current transaction is expired or aborted ... conflict: YB001` |
| With `yb_enable_read_committed_isolation=true` | ❌ still aborts — error changes from `[repeatable read]` to `[read committed]` and reports *"query layer retry isn't possible … some data was already sent to the user"* |

**Conclusion:** the advisory-lock blocker (the user's original issue) is **genuinely resolved**, but
Authentik 2026.5 **does not install cleanly on YugabyteDB 2025.1.4.0** — its Django migration
transactions hit YugabyteDB transaction-conflict aborts (YB001) that even READ COMMITTED can't retry
once the transaction has returned rows. A secondary symptom: a half-migrated schema (django_migrations
populated, `authentik_core_group` absent) then wedges restarts on the `to_2025_12_group_duplicate.py`
system migration with `UndefinedTable`. Getting the full migration suite to pass would need deeper
YugabyteDB transaction tuning and/or app-side accommodations that Authentik does not officially
support (PostgreSQL is its only supported backend). The manifest is kept as an **experimental,
documented-non-working** backend (mirroring `k8s/cockroachdb/`), useful for advancing the
investigation when a newer YugabyteDB / Authentik lands.

## 7. Recommendation for this repo

| Action | Recommendation |
|--------|----------------|
| `k8s/cockroachdb/` | **Keep frozen as-is** (or document-and-archive). It remains non-working and the upstream fix is open/unmilestoned. Renovate already ignores `k8s/cockroachdb/**`, which is correct — there is no point bumping `cockroach:v22.2.19` toward a release that still lacks session-scoped locks. Optionally add a one-line pointer in that dir's notes to [cockroachdb#169981](https://github.com/cockroachdb/cockroach/issues/169981) so a future revisit has the tracking issue. Revisit **only** when #169981 closes (a future 26.x). |
| YugabyteDB | **POC built + run (`k8s/yugabytedb/`); keep as experimental/non-working.** §6a confirmed the advisory-lock blocker is fixed but Authentik's migrations still abort on YB001 transaction conflicts. Like `k8s/cockroachdb/`, the manifest is frozen + Renovate-ignored; revisit when a newer YugabyteDB (better DDL-transaction handling) or an Authentik release tolerant of YugabyteDB's transaction model lands. Do **not** present YugabyteDB as supported. |
| Default datastore | **No change.** PostgreSQL (`postgres:18-alpine`) stays the production-working backend; neither alternative displaces it. |

## 8. References

All accessed **2026-06-26**.

- Authentik — Configuration (PostgreSQL 14–18, "requires PostgreSQL"): <https://docs.goauthentik.io/install-config/configuration/>
- Authentik — Release 2024.6 (PG 14 minimum; advisory locks replace Redis locks for SCIM/LDAP): <https://docs.goauthentik.io/releases/2024.6/>
- Authentik — Release 2026.5 (no new DB requirements): <https://docs.goauthentik.io/releases/2026.5/>
- Authentik — Issue #14562 "Flag to disable advisory locks" (open; 5 lock sites; CockroachDB context): <https://github.com/goauthentik/authentik/issues/14562>
- CockroachDB — Issue #13546 "fill out pg_advisory_lock stubs" (closed; 2017 origin): <https://github.com/cockroachdb/cockroach/issues/13546>
- CockroachDB — Issue #169981 "implement session-scoped advisory locks" (open, opened 2026-05-08, no milestone): <https://github.com/cockroachdb/cockroach/issues/169981>
- CockroachDB — Issue #170014 "advisory lock builtins should respect lock_timeout" (target 26.3.0): <https://github.com/cockroachdb/cockroach/issues/170014>
- YugabyteDB — Explicit locking (advisory locks GA + default since v2025.1): <https://docs.yugabyte.com/stable/explore/transactions/explicit-locking/>
- YugabyteDB — Issue #3642 "[YSQL] Add support for advisory_locks" (closed/Done; `ysql_yb_enable_advisory_locks`): <https://github.com/yugabyte/yugabyte-db/issues/3642>
- YugabyteDB — v2.25 / v2025.1 PostgreSQL 15 rebase (prior versions PG 11.2): <https://www.yugabyte.com/blog/postgresql-15-compatibility-in-yugabytedb/>, <https://docs.yugabyte.com/stable/api/ysql/pg15-features/>
