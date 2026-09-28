# uspace

The core API: files, volumes, quotas and jobs. File contents live in MinIO; who owns
what, permissions and usage live in an embedded [fslite](../../pkg/fslite/README.md);
jobs and apps live in their own SQLite database. Entry point: `cmd/uspace`, config
`configs/uspace.conf`.

## Who is calling

Every `/api/v1` request is authenticated (`identity.go`):

- **users**: a minioth access token (`Authorization: Bearer`). It is verified with
  the configured algorithm only (`JWT_SIGNING_ALG`: HS256 with `JWT_SECRET_KEY`, or
  RS256 against minioth's JWKS), never the one the token's header names.
- **services** (frontapp): `X-Service-Secret`. A service acts for a user through the
  `Access-Target` header: `vid:vname:target uid:gid,gid,...`, with the primary group
  first.

`/api/v1/admin/...` requires an admin token or the service secret.

## Routes

| Route | |
|---|---|
| `GET /resources` | list (the "ls") |
| `POST /resource/upload` | upload; quota claimed before the write, refunded on failure |
| `GET /resource/download`, `/resource/preview` | needs `r` |
| `DELETE /resource/rm`, `PATCH /resource/mv` | needs `w` on the target |
| `POST /resource/cp` | needs `r` on the source; the destination must be free |
| `PATCH /resource/permissions`, `/ownership`, `/group` | owner only |
| `GET,POST /job`, `GET /job/log`, `POST /job/cancel` | submit, list, read the saved log, cancel |
| `GET,POST /app` | the job applications catalogue |
| `admin: /volumes`, `/user/volume`, `/job`, `/app`, `/system-conf`, `/system-metrics` | |

Swagger: `/api/v1/swagger/index.html` (`make api-docs`).

## Jobs

```
submit ──► queue (J_QUEUE_SIZE, full → 503) ──► dispatcher ──► worker (J_MAX_WORKERS)
                                                                 │
                        docker executor (J_EXECUTOR=docker) ◄────┤────► kubernetes executor
```

- A job is an **application** (built-in images from `applications/`, or any catalogue
  entry) or **code** in a language mode (`python`, `node`, `ruby`, `php`, `perl`, `r`,
  `go`, `java`, `c`, with aliases such as `py` or `js`). The code goes into the
  `$LOGIC` environment variable and is never pasted into a shell command, so quotes
  in it are harmless.
- The container gets **presigned URLs only** (`INPUT_URL` for GET, `OUTPUT_URL` for
  PUT, valid for the job's timeout plus a margin), never storage credentials.
  Submission checks that the user can read the input and write the output.
- Output recording: a new output is inserted and charged to the owner, with
  the job owner's primary group. An existing output gets its new size, and only
  the difference is charged or refunded.
- Live output streams through [wss](../wss/README.md). The log is kept (last 64 KiB)
  in `job_logs` and served by `/job/log`.
- `POST /job/cancel` works on queued jobs (skipped at dequeue) and running jobs (the
  container or kubernetes Job is killed). Finished jobs answer 409.
- Each job records the `engine` it ran on (`docker` or `kubernetes`).

The docker executor is for development: it needs the docker socket (root on the
host) and joins `J_DOCKER_NETWORK` so the job can reach MinIO under the name the URLs
were signed for.

## Storage

`StorageSystem` (`storage_system.go`) is the object-store interface. MinIO
(`minio/`) is the one in use, and fslite can stand in. Every method takes a
`context.Context`.

## Tests

`go test ./internal/uspace` runs the following without any cluster or MinIO:

- token and identity checks
- the kubernetes executor, against client-go's fake clientset
- docker run arguments, presigning, output recording, cancellation and quotas

The whole stack is covered by `make smoke`.
