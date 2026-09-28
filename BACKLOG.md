# kuspace BackLog

Where the project stands, what changed and why, and what is left to do.

- **Revisions** - every change made during the September 2026 review and hardening
  sessions (carried out with AI assistance, Claude), grouped by area.
- **Open** - known problems and planned work, most severe first.
- **Keep or drop** - which parts earn their place in the product.
- **Decisions** - questions that need the owner's call.

Severity: `CRIT` a user can act as someone else / read or destroy others' data ·
`HIGH` data loss, a broken core feature, or a real exposure · `MED` fragile or
wrong in edge cases · `LOW` cleanup.

Verification: `make check` (fmt, vet, unit tests) and `make smoke`
(scripts/smoke.sh, an end-to-end test of a running compose stack - every
security fix below has a regression check in it).

---

## Revisions

### Security

| Sev | Where | Problem | Fix |
|---|---|---|---|
| CRIT | frontapp download/preview/delete | Copied every client header into the request to uspace, then *added* its own `Access-Target`. uspace reads the first value, so a user sending `Access-Target: ... 0:0` acted as root: read or deleted anyone's files (reproduced). | Only `Content-Type`, `Accept`, `Range` are forwarded; identity headers are always `Set`. |
| HIGH | uspace upload | Object was written to MinIO before the metadata insert rejected the duplicate name: any user could silently replace any other user's file, even a private one (reproduced). | Name checked first; `409 Conflict` before touching storage. |
| HIGH | uspace move/copy | Destination was never checked; MinIO copy overwrote whatever was there. | Same pre-check, `409` on a taken destination. |
| HIGH | resource listing | frontapp listed files *as root* and uspace did not filter: every user saw every file (names, sizes, owners). | frontapp lists as the caller; uspace returns only what the caller may read. |
| HIGH | jobs | Nothing checked that the submitter may read a job's input or overwrite its output, while the executor uses service credentials: a job could exfiltrate another user's private file. | uspace authorizes input (read) and existing output (write) at submission; frontapp sends the submitter's identity and now passes refusals through (it used to answer 200 to a refused job). |
| HIGH | job listing | frontapp fetched all jobs as root for everyone - other users' code, inputs, outputs. | Non-admins get only their own jobs. |
| HIGH | minioth (dependency) | v1.0.0 logged every user's plaintext password on login (SQLite store). | Fixed upstream, released **minioth v1.0.5**, submodule bumped. |
| HIGH | repository | Secrets (JWT keys, service secret, admin and MinIO passwords) committed to the public repo; minioth's database with bcrypt cost-4 hashes tracked too. | All secrets rotated. They live only in the gitignored `configs/secrets.env` (template: `secrets.env.example`, generator: `make secrets`). `secrets.yaml` and `data/db/` untracked and ignored. Old values stay in git history but are dead. |
| HIGH | frontapp auth middleware | A non-200 from the token check decoded to an empty identity, and `strings.Contains(group, "")` is always true: fail-open on invalid tokens. | Status checked first; validity check is fail-closed. |
| HIGH | uspace permission checks | The access and ownership middlewares found "the resource" with `name LIKE %target%` across all volumes (`_` acting as a wildcard): checks ran against unrelated files - e.g. `o_x.txt` matched `hello_x.txt` - denying owners and checking the wrong file. | Exact lookup by name and volume; `404` when it doesn't exist. Four duplicated lookup blocks became one helper. |
| HIGH | ownership (`IsOwner`) | Any member of a file's group counted as its owner, and ownership authorizes chmod/chown/chgrp. Every user is in the shared `user` group, so a file shared with it could be taken over by anyone. | Owner means the owning user only (root is handled separately), as in Unix; unit-tested. |
| MED | uspace, fslite server | Service authentication was skipped in gin `debug` mode, and the config default mode is `debug`. | Authentication is unconditional. |
| MED | frontapp | Security headers were switched off (the CSP they set broke the inline handlers); HSTS was sent over plain HTTP. | Working CSP enabled always; HSTS and `Secure` cookies only over HTTPS; session cookie `SameSite=Strict`. |
| MED | frontapp / logs | bcrypt hashes forwarded to the browser (`fetch-users?format=json`); `/system-conf` exposed `MINIOTH_SERVICE_SECRETS`; every service printed its full config, secrets included, at startup. | Hashes stripped; secret-named keys filtered from `/system-conf` and redacted in logs (`isSecretName`). |
| LOW | utils | `IsValidPath` accepted `..` segments. | Rejected; first unit test in the repo. |
| LOW | frontend | `index.html` loaded Font Awesome from a CDN. | Uses the vendored copy (and the CSP forbids third-party origins). |

### Correctness

| Sev | Where | Problem | Fix |
|---|---|---|---|
| HIGH | registration | New users never got storage. The volume claim ran in goroutines bound to a context cancelled on redirect, called a route uspace had removed, lacked the `Access-Target` header, passed the wrong type to fslite, and wrote to a misnamed table (`userVolume` vs `user_volume`). | Claim runs synchronously; all five causes fixed. |
| HIGH | fslite | Lookup by resource id selected 1 column and scanned 14: **chmod, chown and chgrp always failed**. | `SELECT *`. |
| HIGH | frontapp copy | Called uspace with PATCH (route is POST) and never sent `dest`: copying never worked. | POST with `dest`. |
| MED | uspace upload | Every file was recorded with volume id 0, a volume that never exists (hid behind SQLite not enforcing foreign keys). | Real volume id resolved from the volume name. |
| MED | uspace user volumes | Quota parsed from the volumes *path* (always 0). | Uses `LocalVolumesDefaultCapacity`. |
| MED | frontapp `/user/me` | minioth v1.0.0 returns an object, frontapp expected an array; an error body decoded to user 0 (root) and was patched. | Status checked; object decoded. |
| MED | minioth v1.0.0 contract | Login/register/introspection responses moved to snake_case. | frontapp adapted. |
| LOW | uspace upload | An upload with no files answered `200 uploaded`. | `400`. |
| LOW | apps catalog | bash image `kuspace/applications-bash-v1` (slash) instead of `:applications-bash-v1`: every bash job would fail to pull. | Fixed in `applications.json` (existing databases keep the old row until edited). |
| LOW | SQLite | No busy timeout; concurrent writes failed with "database is locked". | `_busy_timeout=5000`, WAL. |

### Features

- **Dashboard** (landing page): installed applications as a turning ship's helm
  (pick one, run it), job tiles, status bar and 14-day chart (each with a table
  view), storage meter, recent jobs, quick actions; admins also see platform
  totals, volumes and - on kubernetes - cluster metrics.
- **System page** (replaces Settings): each service's health, response time and
  address; its settings load on demand.
- **Default applications** installed from `internal/uspace/applications/applications.json`
  on start-up (`INSERT OR IGNORE`: admin edits survive restarts).
- **Dark mode** has one source of truth and follows content loaded later (the
  job page's white description box, light tabs and invisible icons are fixed).

### Build, deploy, tooling

- minioth moved out of the repo into its own (`third_party/minioth` submodule, v1.0.5).
- docker-compose made to work: build contexts, config mounts, all services,
  MinIO health check gating uspace, uspace `restart: on-failure`.
- Builder images pinned (`golang:latest` moved to Debian trixie; binaries needed
  a newer glibc than the bookworm runtime).
- New `Makefile`: `setup`, `secrets`, `up`/`down`/`logs`, `smoke`, `check`,
  `lint`, `k8s-*`, `help`. The old one built and started the deleted
  in-repo minioth.
- `scripts/smoke.sh`: end-to-end regression test (76 checks, all passing).
- golangci-lint config migrated to v2 (the installed linter rejected v1).
- `.dockerignore` keeps `.git`, `thesis/` and `data/` out of build contexts.

### Round 3 (uspace, wss, minioth v1.0.6)

| Sev | Where | Problem | Fix |
|---|---|---|---|
| CRIT | kubernetes executor | A panic anywhere in a job's goroutine takes all of uspace down, and three were reachable: an unchecked type assertion on watch events (error events are not Jobs), `resource.MustParse` on user-supplied quotas, and a timer closing the log channel after 500 s while long jobs kept sending. | Events type-checked, quotas parsed with errors (and validated at submission: `400`), one output sink that ignores sends after close. |
| HIGH | kubernetes executor | Watches ending early returned "unknown"; a silent watch blocked forever; short jobs finished before their pod was "ready", so their logs were never streamed; cancel used a different job name than create. | Re-watch until the job's deadline; follow logs of running *or finished* pods; one `k8sJobName`. Unit-tested with client-go's fake clientset. |
| HIGH | wss | No authentication; double `close(client.Send)` panics; a goroutine + open log file leaked per job. | Consumers need a one-minute HMAC ticket from frontapp (issued only to the job's owner or an admin); producers and session deletion need the service secret; same-host origin check; each client's queue closes once; a session shuts down (and closes its log) when its last client leaves. |
| HIGH | minioth (dependency) | Token refresh was broken: login made the refresh token with the *username* as user id, and refresh built the new access token from the refresh token's claims (no username, groups `"not-needed"`). | minioth **v1.0.6**: refresh re-reads the user; login and refresh share one issuer; test now checks what the refreshed token says. |
| HIGH | uspace trust | uspace believed whatever identity `Access-Target` carried (only frontapp could call it). | uspace verifies callers itself: a user's minioth access token (identity taken from the token, header identity overwritten) or the service secret (trusted to state identity). Admin routes: service or `admin` group. uspace can now be used directly - CLI, gshell - without frontapp. |
| MED | quotas | Never enforced through uspace; the old fslite code treated capacity 0 as "full", never compared usage to the quota, and ignored the volume. | `fslite.ClaimSpace/ReleaseSpace` (transactional; 0 = unlimited; root exempt). uspace charges uploads and copies (`507` when over), refunds deletes and failed writes, moves usage on cross-volume moves; job outputs are recorded. |
| MED | SQLite foreign keys | Declared, never enforced (SQLite needs them enabled per connection); bogus volume ids went unnoticed. | Enforced. Every write path records a real volume id; a start-up migration repairs old rows; volume deletion now cascades. |
| MED | job queue | A job was saved, then queued; a full queue left a permanent "pending" row. The batch path published jobs without their ids (all "0"). | Saved and queued together; if queueing fails the row is removed and the client gets `503` + `Retry-After`. Batches return real ids. |
| MED | move | `WHERE name = ?` ignored the volume: moving `/a.csv` renamed every `/a.csv` in every volume; `vid` didn't follow a cross-volume move. | Source volume in the `WHERE`; `vid` follows `vname`. |
| MED | copy | The copy's record had no owner and no permissions: nobody but root could read it. | Owned by the copier, default permissions, primary group, real size/volume. |
| LOW | names | Stored with and without a leading `/`. | One form (`fslite.NormalizeName`) on every write + start-up migration; lookups try one form. |
| LOW | uploads | Group = the user id. | Group = the user's primary group (`pgroup` claim from minioth v1.0.6; frontapp lists it first). |
| LOW | `syncUsers`, `streamToSocket` | Dead code with a nil-cancel call / nil-response dereference. | Fixed; `syncUsers` now runs at start-up (with retries) and gives users registered earlier their volume claim. |
| LOW | fslite server | Unreachable "debug uid 0" fallbacks. | Removed (server kept). |

Also: uspace `handleJob`/`handleJobAdmin` share one query/submit/delete path
(~100 lines less); job output is persisted (`job_logs`, tail up to 64 KiB) and
served by `GET /job/log` (owner or root) - the page falls back to it when the
live stream is over; every SQLite call takes a context (uspace passes the
request's; refunds use an uncancellable one); minioth-contract JSON tags are
marked as such for the linter.

### Round 4 (2026-09-28: jobs)

| Sev | Where | Problem | Fix |
|---|---|---|---|
| CRIT | job pods | Pods ran user code with the MinIO root credentials (`ACCESS_KEY`/`SECRET_KEY`): any job could read, overwrite or delete every user's files. | Presigned `INPUT_URL` (GET) / `OUTPUT_URL` (PUT) for exactly the job's two objects, valid for its timeout + 15 min. Apps use a stdlib helper (`applications/common/kuspace_io.py`); images are `-v2`; start-up moves built-in `-v1` rows to `-v2`. Verified end to end: tampered or repurposed URLs get 403. |
| HIGH | caengine app | Never parsed (every `return` at column 0) and had no defaults. | Fixed; a rejected image fails the job. |
| HIGH | docker executor | Never worked (read the output before starting, mounted a file nothing created, no status/output recording, no built-in apps). | Rewritten on the kubernetes executor's contract; the dev compose runs jobs with it (see Dev-session state). |
| MED | uspace tokens | Only HS256. | `JWT_SIGNING_ALG`: HS256 (shared key) or RS256 (minioth's JWKS, cached by kid, rate-limited refetch). The token's header alg is never trusted (algorithm confusion tested). |
| MED | job cancellation | `CancelJob` was a no-op. | Queued jobs are skipped; running ones are stopped (kubernetes job deleted / container killed) with status `cancelled`. `POST /job/cancel` (owner or root), frontapp `POST /verified/job-cancel`. |
| MED | job outputs | Overwriting an existing output failed to record ("already exists") and kept the old size. | The record is updated and only the size difference is charged/refunded to the file's owner. |
| LOW | jobs | Nobody could tell where a job ran; jobs never showed "running". | `jobs.engine` (docker/kubernetes); status `running` once started. Job queries name their columns (no `SELECT *`). |

### Round 5 (2026-09-28: fslite, context, config, CI, tests)

| Sev | Where | Problem | Fix |
|---|---|---|---|
| HIGH | fslite admin tokens | Signed with the hardcoded key `r4nd0m`; any algorithm accepted. | Key derived from `JWT_SECRET_KEY` (HMAC, fslite-only); HS256, issuer `fslite` and expiry required; no key, no server. |
| HIGH | fslite server | Never worked end to end: every admin upload/delete was 500 (uid = the admin's UUID), uploads carried volume id 0 (foreign key), fractional `JWT_VALIDITY_HOURS` issued expired tokens, duplicates answered 200. | Admins act as uid 0; the volume id is resolved (unknown volume 404); validity computed in float; duplicates 409; quota refusals 507 and refunded on failure. |
| MED | fslite server auth | Wrong service secret got an empty 200; a short `Authorization` header panicked; bad tokens were 500. | All 401; constant-time secret compare. |
| MED | fslite passwords | bcrypt cost 4; odd character rule (no `-`). | Default cost; 8-72 printable characters. |
| MED | fslite names | Check-then-insert race on duplicate names. | `UNIQUE (vname, name)` index decides (`ErrResourceExists`); existing duplicates reported at start. Tested with 20 concurrent inserts. |
| MED | storage | `StorageSystem` calls ran on `context.Background()`; `Insert` returned a CancelFunc callers deferred while nil. | Every method takes the caller's context (handlers pass the request's); `Insert` returns only an error. |
| MED | fslite `Download`/`Stat` | Locality check inverted (worked only without local files); rejected the `*Resource` uspace passes. | Fixed; `Download` returns a func that closes the file. |
| MED | config | Settings silently ignored: conf files and the uspace config map set `DB_FSL_*`, `DB_JOBS_MAX_IDLE_LIFETIME`, `API_LOGS_MAX_SIZE`; `FSL_UNLOCKED` was never applied. CORS lists split with `SplitAfter` (`"GET,"`). `DeepCopy` dropped fields. | Sectioned `EnvConfig` with one tag-driven loader (bad values and missing secrets stop start-up, all listed); keys renamed to the real ones with the values in effect (no data moved); a test fails on any unknown key. |
| MED | CI | `ci-cd.yaml` built `./cmd/service-a` with Go 1.21: could never pass. `./...` broke on root-owned dirs in `data/`. | `.github/workflows/ci.yml` (`make ci`: gofmt, vet, race tests; minioth tests; lint on new issues). `data/go.mod` keeps runtime data out of `./...`. |
| LOW | job outputs | Group = the job owner's uid. | The owner's primary group (from `Access-Target`). |
| LOW | language modes | Code pasted into `python -c '...'` etc.; a `'` broke it. | Code is passed in `$LOGIC`; one table of modes and aliases; unknown languages list the supported ones. |
| LOW | utils | `MakeMapFrom` filtered nothing (checked a `reflect.Value` wrapper). | Fixed. |
| LOW | wss | `J_WS_LOGS_PATH` default was a file name, concatenated with the session file name. | A directory, joined properly. |
| LOW | uspace group volumes | Handler deferred a nil func. | Fixed; the feature itself is unfinished (see Open). |

Also: READMEs for `pkg/fslite`, `internal/uspace`, `internal/wss`; tests for
`authorizeJobIO`, the fslite server (httptest) and wss sessions (real
WebSockets); the config (defaults, strict parsing, shipped files); the old
`tests/fslite` (never ran) replaced; Go experiments moved to
`playground/_go_experiments`.

### Round 6 (2026-09-28: group volumes, frontapp refactor, security)

| Sev | Where | Problem | Fix |
|---|---|---|---|
| CRIT | uspace Access-Target | Split at the first space, and frontapp put user input (download targets, file names) before it: `target=/f.txt 0:0` made the caller uid 0, bypassing every permission check. | Split at the last space; the identity must be `uid:gid[,gid...]`. File names with spaces work now. |
| HIGH | uspace jobs | Any user could list every job (code, inputs, outputs) and pick any uid filter. | Non-privileged callers (not a service, admin or root) see their own jobs only. |
| HIGH | frontapp roles | `strings.Contains("user,admin", group)`: a group named `adm`, or a token with no groups, passed as admin. | Exact group names (`authn.Claims.InGroup`). |
| HIGH | gshell | Lines rendered with innerHTML, including other users' input in the shared room: anyone could run script in every connected page. It also never connected (no wss ticket). | Text nodes; frontapp issues `jack` tickets for the shared room (jid 0) only. |
| MED | frontapp CSRF | None (SameSite only). | Double-submit token (`csrf_token` cookie, `X-Csrf-Token` header via `csrf.js`) plus an Origin check. |
| MED | frontapp | Every request asked minioth to introspect the token (no revocation exists); user calls went to uspace as a service with an identity frontapp assembled from input. | Local verification (`internal/authn`, shared with uspace); calls carry the user's own token. |
| MED | frontapp fetch-volumes | Listed every volume, as root, to every user. | Admins: all; users: default volume + their group volumes. |
| MED | fslite | Package globals (data path, token key, capacity) overwritten by every instance; volume dirs created 0644; the verbose log printed the admin's bcrypt hash. | Per-instance fields (`tokenSigner`, `dataPath`); `objectPath` keeps names inside the data dir; 0750; logs removed. |
| LOW | frontapp | Logout deleted whatever cookies its query named; `jsonPostRequest` cancelled its context before callers read the body; users added by an admin got no storage until uspace restarted; login stored tokens it couldn't verify. | Fixed. |

Features: **group volumes** - a volume shared by one group: members only
(403 otherwise), files get the group's gid, usage charged to the group's
quota (never the members' own). fslite `group_volume` table + accounting,
uspace `/admin/group/volume` and `/volumes/shared`, job outputs honour it.

frontapp: `handlers.go` (3.3k lines) split into storage / jobs /
admin_users / account / pages over one upstream client; ~2.3k lines incl.
tests (was 4.4k), with httptest suites (sessions, CSRF, storage, jobs,
accounts). Config: `LoadConfig(path, sections...)` - each service loads,
checks and logs only its sections (Tokens and Storage split out).

### Round 7 (2026-09-28: consistency, errors, edges, web refurbish)

| Sev | Where | Problem | Fix |
|---|---|---|---|
| HIGH | quotas | Usage counters adjusted by separate claim/release calls; any failure or crash between a record change and its counter left usage wrong for good. | Usage is the sum of the record sizes (views `volume_usage`, `user_volume_usage`); `InsertResource` checks the quota and inserts in one `BEGIN IMMEDIATE` transaction (20 concurrent 0.2 GB inserts into 1 GB: exactly 5). |
| MED | uspace writes | Delete removed the object before the record: a failure left a listed file with no data. | Rule: object first/record last on create, record first/object last on delete - failures leave only invisible orphan objects (for `fsck`, next). |
| MED | fslite | Deletes matched the raw name ("a.txt" never matched "/a.txt") and reported success; a query read a table that doesn't exist. | Normalized; a delete that matches nothing is `ErrResourceNotFound`. |
| MED | errors | Ten places decided what happened by reading error text ("already exists", "not empty", "empty", "unique"). | Shared kinds (`ut.ErrNotFound`, `ErrExists`, `ErrNotEmpty`, `ErrNoSpace`, `ErrUnavailable`, ...), fslite and the MinIO client wrap them, one `HTTPStatus` mapping; missing object 404 (was 400), taken name 409 (was 422), MinIO down 503. |
| MED | uspace listing | A user with no files got 404 (shown as an error). | Empty list. |
| MED | shutdown | uspace closed its databases before draining requests; running jobs lost their status. | Requests (10s), then jobs (15s), then databases; new jobs 503 while draining; compose grace 30s. |
| MED | edges | A flat 5-minute client timeout (cut big downloads, let a dead service hang a page); no readiness; unreachable services answered 502 JSON on page loads. | Per-phase timeouts, `/readyz` everywhere (+ k8s probes), 503 + Retry-After and the error page. |
| MED | csrf.js | htmx goes through XHR, so the header was sent twice ("tok, tok"): every htmx write refused. | Set once. |
| - | web | Refurbish merged (`a1f1164`): new design in both themes, no inline code, 8 XSS spots fixed; CSP is now `script-src 'self'; style-src 'self'`. | |

### Round 8 (2026-09-28: LOW items, quotas & sharing UI, languages as apps)

| Sev | Where | Problem | Fix |
|---|---|---|---|
| LOW | fslite | Prefix listings matched "%name%" across every volume; "_"/"%" were wildcards. | Path prefix within the volume, wildcards escaped. |
| LOW | uspace | `system-metrics` panicked (500) without kubernetes. | Says so (`available: false`) and reports uspace's own process; dashboard shows the note. |
| LOW | web | Duplicate element ids (search-by x7, ...), add-user password in plain text, job sort ignored two columns. | Classes, a password field, every column sorts. |
| LOW | minioth | `/admin/users` reported pgroup = uid. | The group's gid is recorded; old users repaired at start (branch `next`, `254d20f`). |
| LOW | minioth plain store | Truncating rewrites, one-process lock, unchecked writes, ':' in values. | Atomic rename, flock, ordered/checked writes, values refused (`da09dba`). |
| - | gShell | An echo room. | A shell over the APIs: ls, cat, rm, volumes, jobs, log, cancel, whoami; `say` for the room. |
| - | UI | No way to manage personal quotas or group volumes. | Admin > Storage > Quotas & sharing (uspace `/admin/user/volume` GET/PATCH/DELETE, frontapp routes). |
| - | languages | Code languages had no UI and defaulted to `:latest` images. | Catalogue apps with pinned small images and working starters (all nine run against presigned URLs). |

### Round 9 (2026-09-28: code health)

| Sev | Where | Problem | Fix |
|---|---|---|---|
| MED | fslite listings | Fuzzing found prefix listings case-insensitive (SQLite LIKE: "AB" listed "/ab"). | Exact `substr` comparison. |
| MED | uspace moves | The old object was removed before the record update: a failed update left a record pointing at nothing. | Copy, update the record, then remove the old object. |
| LOW | lint | 42 findings; CI only failed on new ones; `unused` disabled. | Zero findings, CI fails on any, `unused` on: 18 dead functions removed (one would `log.Fatal` uspace). |
| LOW | errors | `[ERROR]`-prefixed messages without a kind (500 for bad input). | Kinds (`ErrInvalid`, ...), helpers deleted. |

Tests added: 9 fuzz targets (`make fuzz`, CI 10s each; Access-Target
identity, names/paths, tokens, tickets, config values, prefix listings);
property tests - quota accounting against a model (8 seeds x 300 steps)
and "no record without its object" through the handlers over storage that
fails 30% of calls before or after taking effect (6 x 250); both fail on
the bugs they guard (checked by planting them). Browser JS suites in
`web/tests` (csrf.js, gShell commands, every language starter against
fake presigned URLs; `make test-js`, in CI with ruby/php/java installed).
tests/uspace (thesis scaffolding) retired to playground.

minioth v1.1.0 (submodule bumped): frontapp now uses its user-facing
endpoints instead of the service secret - password change is `POST
/v1/passwd {current_password,new_password}` with the user's token (the
extra admin verify-password call is gone) and ends the session; email is
`PATCH /v1/user/me`; logout calls `POST /v1/logout`, revoking the user's
tokens everywhere. `internal/authn` asks minioth's introspection whether a
token was revoked (`JWT_REVOCATION_CHECK`, default on): answers cached 30s,
revoked ones until expiry, accepted on the signature while minioth is
unreachable; the service that revokes a token forgets its cached answer at
once. Smoke checks the old session and password stop working.

---

## Open

### Security

### Correctness and robustness
- `HIGH` **consistency checker (`fsck`)** - steps 3-4 of the plan: compare
  buckets with records (orphan objects older than 1h, records without
  objects, size drift, volume halves), report periodically, repair on
  `--repair`/admin call; fault-injecting storage fake + property tests; fsck
  in smoke. Also: move/copy ordering audit.
- `HIGH` **jobs across restarts**: queued jobs stay "queued" and running ones
  "running" after a restart (drain now leaves them so) - re-queue / re-attach
  or fail them at start, plus a timeout watchdog.
- `PENDING` minioth branch `next` (primary-group fix, plain-store hardening)
  is committed locally only (rebased on v1.1.0): push it and tag v1.1.1,
  then bump kuspace's submodule to it.

### Code health
- `LOW` a frontend (JS) linter and DOM-level tests for the console pages
  (quotas.js, admin-panel.js) - the JS tests cover csrf.js, gShell and the
  starters only.
- `LOW` the Swagger docs (api/) predate this year's API changes; regenerate
  with `make api-docs` after the next API change settles.

### Operations
- `MED` CI runs unit tests and lint (`.github/workflows/ci.yml`) but has not run
  on GitHub yet (first push will tell). `make smoke` on a compose stack
  nightly is still to do.
- `MED` the kubernetes executor is unit-tested (fake clientset) but not run
  end to end in a cluster yet (kind/minikube are available locally); the
  docker executor is verified end to end by `make smoke`.
- `LOW` runtime data lives in `data/` inside the repository (kept out of the Go
  build by `data/go.mod`). A deployment should mount it elsewhere.
- `LOW` compose images must be built one at a time on 8 GB machines (`make images` does).

### Pending actions
- **History rewrite** (postponed): scrub the old, rotated secrets from all
  branches - including `main` and `thesis` - and force-push. Redo it from
  the current local repository (the copy prepared earlier predates these
  commits).

### Next up
1. `fsck` (consistency step 3): the fault-injecting storage fake and the
   property tests are in place.
3. Jobs across restarts.
4. Schema migrations (see Design notes below).
5. minioth plain store: harden or drop (decision pending).

### Design notes: schema migrations
Today schema changes are applied by start-up code scattered across the
services (`CREATE TABLE IF NOT EXISTS`, name normalization, volume-id
repair, the `jobs.engine` column). Nothing records what has run, so every
change needs its own "is it already applied?" check.

Plan - a small runner in `internal/utils` (no new dependency):
- each database (uspace's jobs.db, fslite's db) owns an ordered list of
  migrations embedded with `go:embed` (`migrations/0001_init.sql`,
  `0002_job_engine.sql`, ...), plus Go functions for data fixes SQL can't
  express;
- a `schema_migrations(version, name, applied_at)` table records what ran;
  at start-up each pending migration runs in its own transaction, in order,
  and the service refuses to start if one fails;
- existing databases are baselined: `0001` is today's schema written with
  `IF NOT EXISTS`, so running it on an existing database is a no-op, and the
  current ad-hoc fixes become `0002...` (each already idempotent);
- forward-only (SQLite can't drop columns cheaply); a bad migration is fixed
  by a new one; a backup of the file is taken before the first pending one;
- tests run every migration on an empty database and on a snapshot of the
  previous schema.
minioth needs the same (its BACKLOG already discusses it).

### Dev-session state (revert before release)
- `deployments/docker-compose/docker-compose.yml` uses the external `rumie-minio`
  instead of the bundled MinIO (blocks marked `DEV SESSION`), and
  `MINIO_SECRET_KEY` in `configs/secrets.env` is rumie-minio's password.
- The same file runs jobs on the host's docker engine (`J_EXECUTOR=docker`,
  docker socket mounted into uspace = root on the host). Development only.

---

## Keep or drop (decided 2026-09-27)

| Part | Decision | Notes |
|---|---|---|
| uspace, frontapp, wss, minioth | keep | the product |
| fslite as a library | keep | uspace's metadata and permission store |
| fslite standalone server | keep | reusable on its own; candidate for its own repository after polishing (see Future) |
| OAuth in frontapp (`oauth.go`) | keep, dormant | not wired in; future work for a production launch (placeholder credentials, constant `state`, malformed user-info calls, nobody gets logged in) |
| gshell + wss `jack` role | keep, dormant | future: a CLI-like shell over the uspace APIs |
| `jobs_kafka_dispatcher.go` | keep | placeholder for a pub/sub job backend (see Future) |
| `playground/` | keep | scratch space, not part of the build |
| `build.ps1` | keep | rewritten to mirror the Makefile |
| `cmd/secret_generation` | keep | general-purpose tool |
| `scripts/image_builder.sh` | **dropped** | duplicated `kuspacectl -build` |
| `web/static/js/pie_chart.js` | **dropped** | only used by the old dashboard |
| `data/jwks/jwks.json` (+ `Jwks` config field) | **dropped** | nothing read it since minioth moved out |
| `thesis/` | **moved** | lives only on the `thesis` branch |

## Future

- **pub/sub job backend** (kafka or similar) instead of the in-process queue:
  distributed execution, jobs survive uspace restarts.
- **gshell**: a shell/CLI over the uspace APIs.
- **OAuth / social login**, through the identity provider, for a production launch.
- **fslite as its own repository**: polish the standalone server and its docs.

## Decisions

Decided 2026-09-27:
- Default file permissions are **owner + group** (`rw-r-----`, `DefaultFilePerms`).
- minioth's v1.0.0 and stray `list` tags are deleted (current: **v1.0.6**).
- Git history will be rewritten to scrub the old (rotated) secrets (postponed).
- wss authentication: short-lived tickets from frontapp (implemented).
- uspace verifies minioth tokens itself (implemented).

- Job pod credentials: presigned URLs (done 2026-09-28).
