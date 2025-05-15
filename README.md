# Modular Batch Processing Platform

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

## Development Instructions

[DEV]
Run locally:
> either build and run each service as follows:
>
    make all 

## Brief Description

[BriefDescription]

## A SOA

- identity/storage provision  
- central API for submitting "jobs"  
  - user defined orchestration  
  - code as jobs execution  
  - builtin applications (modular)  
- websocket streaming for logs/resuls/output  
- frontend application for i/o + management  

## Detailed Information

[Details]

- storage provider (configurable)
  - minio  
            (defaults creds for its builtin management gateway)  
            - minioadmin  
            - minioadmin  
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

- (central) uspace API for accessing + using everything

- frontend application
