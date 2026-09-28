# fslite

A small file-metadata store on SQLite: volumes, resources (files) with Unix-like
ownership and permissions, per-user quotas on each volume. It is used in two ways:

| Mode | How | Who uses it |
|---|---|---|
| **Embedded** (metadata only) | `fslite.NewFsLite(cfg)` with `FSL_SERVER=false`, `FSL_LOCALITY=false` | uspace: file contents live in MinIO, fslite holds who owns what, permissions and quota usage |
| **Standalone** | `go run ./cmd/fslite` with `configs/fslite.conf` (`FSL_SERVER=true`, `FSL_LOCALITY=true`) | an admin-only HTTP API that also keeps the file contents on local disk |

## Data model

```
volumes       vid, name (unique), path, capacity (GB), usage (GB), createdAt
resources     rid, vid → volumes, vname, name, type, size, perms "rw-r-----",
              uid, gid, timestamps; UNIQUE (vname, name)
user_volume   vid, uid, quota (GB), usage (GB)   one row per user per volume
user_admin    the standalone server's admin accounts (bcrypt)
```

Names are stored normalized (`NormalizeName`: no leading `/`). Foreign keys are on,
so a resource must point at an existing volume.

## Go API

Every storage method takes a `context.Context` first; handlers pass the request's.

```go
fsl := fslite.NewFsLite(cfg)
err := fsl.CreateVolume(ctx, ut.Volume{Name: "data", Capacity: 10})
err  = fsl.Insert(ctx, ut.Resource{Name: "a.csv", Vname: "data", VID: vid, UID: 1000, GID: 1000})
res, err := fsl.SelectObjects(ctx, map[string]any{"name": "a.csv", "volume": "data"})
err  = fsl.ClaimSpace(ctx, uid, "data", sizeBytes, defaultQuotaGB, true) // quota check + charge, one transaction
```

`Insert` accepts a `ut.Resource`, `[]ut.Resource`, `ut.UserVolume` or
`[]ut.UserVolume`. Errors worth matching with `errors.Is`:

| Error | Meaning |
|---|---|
| `ErrResourceExists` | the name is taken in that volume (the unique index decides, so concurrent inserts of one name leave exactly one row) |
| `ErrVolumeExists` | a volume with that name exists |
| `ErrQuotaExceeded`, `ErrVolumeFull` | `ClaimSpace` with `enforce` refused the write |

Concurrency comes from SQLite, not from Go locks: the unique index settles
duplicate names, and quota changes run in one transaction each.

## Standalone server

`POST /login` (JSON `{"username", "password"}`) returns an admin token: HS256,
issuer `fslite`, signed with a key derived from `JWT_SECRET_KEY`. The server refuses
to start without that key. The `/admin/...` routes require the token or the service
secret (`X-Service-Secret`):

- volumes: `new`, `get`, `delete`
- resources: `get`, `stat`, `upload`, `download`, `copy`, `delete`
- `register`, `user/volumes`, `system-conf`

Both kinds of caller act as uid 0 (root). Admin passwords: 8–72 printable characters,
bcrypt default cost. `pkg/fslite/server_test.go` exercises the API through `httptest`.

## Configuration

| Variable | Default | |
|---|---|---|
| `FSL_DB`, `FSL_DB_PATH` | `database.db`, `data/db/fslite` | uspace overrides the file name to `fsl_local.db` |
| `FSL_SERVER` | `true` | serve the HTTP API |
| `FSL_LOCALITY` | `true` | keep file contents under `LOCAL_VOLUMES_DEFAULT_PATH` |
| `FSL_ACCESS_KEY`, `FSL_SECRET_KEY` | `fsladmin` | the first admin account: **set the password** |
| `FSL_UNLOCKED` | `false` | skip quota and capacity checks on uploads |
| `LOCAL_VOLUMES_DEFAULT_CAPACITY` | `20` | GB; also the default per-user quota |

All variables are listed with their defaults in `internal/utils/config.go` (`FsliteConfig`).

## Known quirks

- Prefix lookups (`SelectObjects` with `prefix`) match names with `LIKE` across all
  volumes.
- Group volumes have query code but no table and no route yet (BACKLOG).
- Schema changes are ad-hoc `ALTER TABLE`s at startup until the migration framework
  in BACKLOG lands.
