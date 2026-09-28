# Kuspace

## A system platform that provides modular batch processing applications for users to run on an end system

### Development Instructions

[DEV]
First-time setup (minioth submodule, fresh secrets, compose env):

    make setup

Run the whole stack locally with docker-compose and check it end to end:

    make up
    make smoke

The web interface is then at <http://localhost:18080>.
Everything else is listed by

    make

use scripts/kuspacectl.go to deploy/destroy/build on kubernetes (or `make k8s-*`)

    go run scripts/kuspacectl.go -h

### Description

Users log in through a web interface, upload data, and run "jobs" on it:
either one of the builtin applications (duckdb, pandas, octave, ffmpeg, bash, ...)
or their own code. Jobs execute on kubernetes (or docker), and their output
streams back live.

### Microservices

- identity provision (minioth)  
- central API for storage and for submitting "jobs" ([uspace](internal/uspace/README.md))  
  - user defined orchestration  
  - code as jobs execution  
  - builtin applications (modular)  
- websocket streaming for logs/results/output ([wss](internal/wss/README.md))  
- frontend application for i/o + management (frontapp)  

### More

- storage provider (configurable)
  - minio  
            (bundled in docker-compose; its root password comes from the secrets file)  
    or
  - [fslite](pkg/fslite/README.md) [custom implementation]
            (a pretty basic fs storing mechanism, with an api and a database holding file metadata)

- minioth (identity provider) [custom implementation, own repository]

    developed at <https://github.com/kyri56xcaesar/minioth>, pinned here as the
    git submodule `third_party/minioth` (currently v1.0.6)

    admin account: `kuspaceadmin`, password = `MINIOTH_SECRET_KEY` from `configs/secrets.env`

    storage as

             - 1. database as storage (sqlite3)
             - 2. plain text as storage (passwd,group,shadow)

- job scheduling mechanism (modular/configurable)
      - simple queue (default)

- job execution system (modular/configurable)
      - kubernetes
      - docker

- (central) uspace API for accessing + using everything

- frontend application (dashboard, job browser, file management, admin panel)

- random data generation and secret generation tools

### Configuration & secrets

- `configs/*.conf` hold the non-secret settings of each service
- every secret (JWT keys, service secret, passwords) lives only in
  `configs/secrets.env`, which is gitignored; `make secrets` creates it with
  random values from `configs/secrets.env.example`
- docker-compose reads it through `deployments/docker-compose/.env`, and
  kuspacectl turns it into the kubernetes Secrets

### Documentation

documentation tools
> go install github.com/swaggo/swag/cmd/swag@latest
>
> go install github.com/go101/golds@latest

generate documentation using:

- make code-docs

- make api-docs

changes, known issues and planned work are tracked in [BACKLOG.md](BACKLOG.md)

### Goals

>
> - ease of access to an end system (kubernetes)
>
> - user environment
>
