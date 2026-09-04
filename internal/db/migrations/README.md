# Migrations: rollback-safety rules

This directory holds the app's forward-only, `go:embed`-embedded SQL migrations
(`internal/db/migrate.go`'s `RunMigrations`, run at boot). Before adding or editing a
`.sql` file here, read this file. It documents two related but distinct rules, the
invariant they exist to protect, a worked example already in this tree, a pre-merge
checklist, and exactly what `cmd/migration-check` enforces automatically versus what
stays a human rule.

## The two rules

These are two different failure modes with two different audiences. Do not collapse
them into one "no destructive DDL" rule — they break different things, at different
points in the release lifecycle.

**Rule 1 — backward-incompatible changes break rollback.** Rolling the app back means
running the *previously-released* binary against the schema HEAD already migrated to.
That binary boots and runs `RunMigrations` too — it just has fewer `.sql` files
embedded than the schema it finds. If a migration shipped in the same release that
stops using some column or table also `DROP`s, `RENAME`s, narrows the type of, or adds
a `NOT NULL`/`CHECK` constraint onto that same object, the rollback binary can no
longer read or write it, and the rollback that was supposed to be the safety net
becomes the outage. This is a **backward-incompatible** change.

**Rule 2 — unsafe-forward changes break or lock the deploy itself.** Adding a `NOT
NULL` column with no `DEFAULT`, running a non-`CONCURRENTLY` index build, or any other
table-rewriting `ALTER` never breaks the *old* binary — the old binary never asked for
that column. But it can fail outright or hold a long lock against a populated table
during the deploy that applies it, independent of rollback. This is an
**unsafe-forward** change, and it can strike even a first-ever release of a table.

## The N-1 invariant

Rollback in this project means: redeploy the immediately-previous released image
against the schema HEAD's migrations produced. That previous release's binary must run
cleanly — reads, writes, and boot migration alike — against HEAD's schema. The
rollback target is always exactly **one hop back (N-1)**, never N-2 or earlier; nothing
in this project's tooling verifies a multi-hop rollback, and none is planned.

Two mechanisms prove this holds, on every relevant run:

- The **`n1-boot` CI job** boots the actual previously-released image against a
  database migrated to the current branch's schema on every migration-touching PR, and
  fails the build if that old binary can't start and stay healthy against real read and
  write traffic.
- An **`internal/db` ahead-of-source test** proves, on every CI run (not just migration
  PRs), that a binary whose embedded migration set is behind the database's applied
  version no-ops cleanly at boot instead of erroring.

A direct consequence: the **expand half and the contract half of a change must never
ship in the same release**. A `DROP` or `RENAME` is only safe once the release that
stopped using the old object is provably not the release a rollback could land on
anymore — i.e., once it's no longer N-1.

## Expand, backfill, contract: a walkthrough already in this tree

This repository already carries one full cycle of the pattern, so it isn't a
hypothetical:

1. **Expand** — `000006_events_watched_artist_name.up.sql` adds
   `events.watched_artist_name` as a plain nullable column, no `DEFAULT`, no backfill.
   Every existing row gets `NULL`; the old binary (which has never heard of this
   column) keeps reading and writing `events` exactly as before. Purely additive.
2. **Backfill** — `000007_backfill_events_watched_artist_name.up.sql` is a
   `WHERE watched_artist_name IS NULL`-scoped `UPDATE`, so it only ever touches rows
   that still need it: re-running it is a no-op, and on a from-scratch database with an
   empty `events` table it updates zero rows. See the migration's own header comment
   for the full derivation of *why* that `UPDATE` is idempotent.
3. **Contract** (not yet done) — a future release could drop whatever old path
   `watched_artist_name` superseded, but only once the release that shipped the expand
   half is no longer the N-1 rollback target — i.e., once at least one more release has
   shipped and become the new rollback point.

The migration files carry the SQL detail; this is only the shape.

## Before you merge a migration

Copy this list into your own review pass before opening a PR that touches
`internal/db/migrations/`:

- [ ] Can the *currently-deployed* version's binary run against the schema this
      migration produces? If not, split the change into an expand-only release now and
      a contract-only release later.
- [ ] The migration is up-only, additive, and lands in the embedded `migrations/`
      directory alongside its `.down.sql` (which the app never runs, but which should
      still exist for the pair).
- [ ] New columns are nullable or carry a `DEFAULT` — never `NOT NULL` with neither.
- [ ] No `DROP COLUMN`, `DROP TABLE`, `RENAME`, type-narrowing `ALTER COLUMN ... TYPE`,
      or new `NOT NULL`/`CHECK` constraint lands in the same release the code stops
      relying on the old shape.
- [ ] No `CREATE INDEX CONCURRENTLY` (or any other statement that can't run inside a
      transaction) in a boot migration — golang-migrate wraps each migration file in a
      transaction, and a concurrent index build cannot run inside one. This stays a
      written rule; no automated check catches it (see below).
- [ ] Any backfill is idempotent and a no-op on a fresh/empty database — scope its
      `WHERE` clause the way `000007` does.
- [ ] Migrations are applied in strictly ascending version order, one file per version
      number, and a migration file that has already shipped in a release is
      **immutable** — never renumbered, never edited in place. `cmd/migration-check`
      treats a rewritten released migration as its own hard error.

## What `cmd/migration-check` enforces

`cmd/migration-check` is a stdlib-only Go CI guard that scans every branch-new
migration file and turns the build red on two distinct finding classes, each carrying
a class-specific message that names the rule above and points back at this file:

1. **backward-incompatible** findings — `DROP COLUMN`, `DROP TABLE`, `RENAME` (column
   or table), and type-narrowing `ALTER COLUMN ... TYPE`, `SET NOT NULL`, or `ADD
   CHECK` against an existing column.
2. **unsafe-forward** findings — `ADD COLUMN ... NOT NULL` with no `DEFAULT` in the
   same clause.

A migration that genuinely needs to ship a destructive change carries an explicit,
file-scoped annotation comment that both documents intent and is machine-checked:

```sql
-- migration-check:allow-destructive expand-shipped-in=v1.7.0 reason=events.old_col superseded by watched_artist_name
```

Both `expand-shipped-in` (a real, shape-validated release tag) and `reason` (free
text) are required; a half-written annotation is a hard error naming the missing key,
not a silent pass-through. `cmd/migration-check` echoes the tag and reason into its
output so an approval trail travels with the migration file itself.

The annotation documents intent — it does **not** override the guard's previous-release
query cross-reference. That cross-reference reads the immediately-previous release
tag's `queries/*.sql` and, for every `DROP`/`RENAME` in a branch-new migration, checks
whether that release's queries still reference the object being removed. If they do,
the finding stays red **regardless of the annotation**, because an object still queried
by the N-1 binary is a live rollback break the author cannot wave through by asserting
intent alone — only the N-1 boot job's actual "does the old binary still run" check
(or a later release) resolves it.

`cmd/migration-check` does not enforce the `CREATE INDEX CONCURRENTLY` boot-migration
rule or general "no blocking DDL" — those stay written-rule-only, per the checklist
above; the guard's scope is destructive DDL and the query cross-reference, not lock
behavior.
