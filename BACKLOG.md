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
- `scripts/smoke.sh`: end-to-end regression test (67 checks, all passing).
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

---

## Open

### Security
- `CRIT` **job pods get the MinIO root credentials** (`ACCESS_KEY`/`SECRET_KEY`
  env) and run user code: any job can read, overwrite or delete every user's
  files, whatever uspace authorized. Fix: per-job scoped access - presigned GET
  for the input and presigned PUT for the output (the application images read
  URLs instead of bucket/object + keys), or a per-job MinIO service account
  restricted to those two objects. Decided: presigned URLs (Next up #1).
- `MED` uspace verifies HS256 tokens with the shared key. If minioth switches to
  RS256, verify against its JWKS (`/v1/.well-known/jwks.json`) instead.
- `MED` no CSRF tokens: `SameSite=Strict` covers modern browsers; add tokens for
  depth once the frontend is refactored.
- `LOW` CSP still needs `'unsafe-inline'` scripts for the templates' `onclick=`
  handlers - remove with the frontend refactor.
- `LOW` fslite's admin password may not contain `-` (odd validation rule).

### Correctness and robustness
- `MED` job outputs: an existing output object is overwritten by the job but its
  record isn't updated (size/time); output files get group = uid (the job
  doesn't carry the owner's primary group).
- `LOW` `CancelJob` in the job manager is a no-op (no queued-job cancellation).
- `LOW` minioth's plain-file store and fslite have their own quirks (see their
  repos); gshell (`jack` role) needs a ticket nobody issues yet - dormant by design.

### Code health
- `HIGH` **frontapp refactor** (planned): `handlers.go` is 3.3k lines of
  hand-built HTTP calls to uspace, each setting `Access-Target` and the service
  secret itself - which is how the header-forwarding bug happened. Replace with
  one typed uspace client that forwards the user's token (uspace now trusts
  tokens); split handlers by domain; drop inline `onclick` for a strict CSP.
- `MED` config: keep one loader, but a small shared `Common` struct (address,
  port, gin mode, service secret, JWT key, logging) embedded in per-service
  structs, instead of one 80-field `EnvConfig` every service loads and dumps.
  Do it with the frontapp refactor.
- `MED` `StorageSystem` methods take no context, so fslite's (now context-aware)
  queries get `context.Background()` from them; add ctx to the interface.
- `MED` tests: unit suites for utils, uspace (identity, executor, access
  targets), fslite (quotas, names, moves, foreign keys), frontapp (gid order),
  minioth; `scripts/smoke.sh` end to end. Next: `authorizeJobIO` and quota
  enforcement through uspace handlers with `httptest`, frontapp handlers.
- `LOW` golangci-lint: 39 style findings left (revive, staticcheck quick-fixes,
  testpackage, ...).

### Operations
- `MED` no CI. `make check` + `make lint` on every push; `make smoke` on a
  compose stack nightly.
- `MED` the kubernetes executor is unit-tested but not yet run end to end in a
  cluster this round (kind/minikube are available locally).
- `LOW` `go build ./...` breaks when containers leave root-owned dirs in `data/`
  (the Makefile builds `./cmd/... ./internal/... ./pkg/...`). Move runtime data
  out of the module tree.
- `LOW` compose images must be built one at a time on 8 GB machines (`make images` does).

### Pending actions
- **History rewrite** (postponed): scrub the old, rotated secrets from all
  branches - including `main` and `thesis` - and force-push. Redo it from
  the current local repository (the copy prepared earlier predates these
  commits).

### Next up (requested 2026-09-28)
1. Job pods: presigned URLs for the input and output instead of MinIO root
   credentials (executor + the six application images).
2. uspace verifies tokens with the configured algorithm: HS256 (shared key)
   or RS256 (minioth's JWKS) - never the algorithm the token names.
3. A job writing an existing output updates that record (size, time) and
   charges only the size difference.
4. Operational job cancellation: queued jobs are skipped, running ones are
   stopped and their kubernetes job deleted; owner or admin.
5. README for `pkg/fslite`; notes on minioth's plain-file store and fslite's quirks.
6. frontapp issues tickets for the gshell room (`jack` role).
7. Per-service config structs embedding a shared `Common` struct.
8. `StorageSystem` methods take a context; a UNIQUE (volume, name) index
   instead of a mutex against duplicate-name races.
9. CI (`make check`, `make lint` on push) so the Makefile and scripts can't
   rot unnoticed.

### Dev-session state (revert before release)
- `deployments/docker-compose/docker-compose.yml` uses the external `rumie-minio`
  instead of the bundled MinIO (blocks marked `DEV SESSION`), and
  `MINIO_SECRET_KEY` in `configs/secrets.env` is rumie-minio's password.

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

- Job pod credentials: presigned URLs (see Next up).
