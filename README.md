# ######################################################

## A system platform that provides modular batch processing applications for users to run on an end system

> **[goals]**
>>
>> - ease of access to an end system
>>
>> - sense of user environment
>
> **[future_goals]**
>
>> - integrated shell as cli/webcli


# ######################################################

[DEV]
Run locally:
> either build and run each service as follows:
>
    make all 

# ######################################################

[BriefDescription]

## A SOA

- identity/storage provision <br>
- central API for submitting "jobs" <br>
  - user defined orchestration <br>
  - code as jobs execution <br>
  - builtin applications (modular) <br>
- websocket streaming for logs/resuls/output <br>
- frontend application for i/o + management <br>

# ######################################################

[Details]

- storage provider (configurable)
  - minio <br>
            (defaults creds for its builtin management gateway)<br>
            - minioadmin <br>
            - minioadmin <br>
    or
  - fslite [custom implementation]
            (a pretty basic fs storing mechanism, with an api and a database holding file metadata)
            (*NOT FULLY FUNCTIONAL YET*)
            (would be nice to use redis or sth ram oriented for this)

- identity provider
      - minioth
        default creds for admin:
        - miniothadmin
        - miniothadmin

- job scheduling mechanism (modular/configurable)
      - simple queue (default)

- job execution system (modular/configurable)
      - kubernetes
      - docker

- (central) userspace API for accessing + using everything

- frontend application
