# Kuspace

## A system platform that provides modular batch processing applications for users to run on an end system

### Development Instructions

[DEV]
Run locally:

use the make file to debug @read Makefile

    make all 

use scripts/kuspacectl.go to deploy/destroy/build

    go run scripts/kuspacectl.go -h

### Description

### Microservices

- identity/storage provision  
- central API for submitting "jobs"  
  - user defined orchestration  
  - code as jobs execution  
  - builtin applications (modular)  
- websocket streaming for logs/resuls/output  
- frontend application for i/o + management  

### More

- storage provider (configurable)
  - minio  
            (defaults creds for its builtin management gateway)  
            - minioadmin  
            - minioadmin  
    or
  - fslite [custom implementation]
            (a pretty basic fs storing mechanism, with an api and a database holding file metadata)

- minioth (identity provider) [custom implementation]

    default creds for admin: (miniothadmin, miniothadmin)

    storage as

             - 1. database as storage (sqlite3/duckdb)
             - 2. plain text as storage (passwd,group,shadow)

- job scheduling mechanism (modular/configurable)
      - simple queue (default)

- job execution system (modular/configurable)
      - kubernetes
      - docker

- (central) uspace API for accessing + using everything

- frontend application

- random data generation and secret generation tools

### Documentation

documantation
> go install github.com/swaggo/swag/cmd/swag@latest
>
> go install github.com/go101/golds@latest

generate documantation using:

- make code-docs

- make api-docs

### Goals

>
> - ease of access to an end system (kubernetes)
>
> - user environment
>
