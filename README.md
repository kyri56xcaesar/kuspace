# ######################################################
A system platform that provides modular batch processing applications for users to run on an end system. 

[goals]
> - ease of access to an end system
> 
> - sense of user environment

[future_goals]
> - integrated shell as cli/webcli
> 
> - ... 


# ######################################################
[DEV]
Run locally:
> either build and run each service as follows:
> 
    make all 



# ######################################################
[BriefDescription]
# A SOA 

+ identity/storage provision 
+ central API for submitting "jobs"
    + user defined orchestration
    + code as jobs execution
    + builtin applications (modular)
+ websocket streaming for logs/resuls/output
+ frontend application for i/o + management


# ######################################################
[Details]
- storage provider (configurable)
     - minio
            *(perhaps you need to kubectl port-forward its ports)*
            defaults creds for its builtin management gateway: 
            - minioadmin
            - minioadmin
    or
     - fslite [custom implementation]
            (a pretty basic fs storing mechanism, with an api and a database holding file metadata)
            (*NOT FULLY FUNCTIONAL YET*)
            (would be nice to use redis or sth ram oriented for this)

- identity provider
      - minioth
        by "https://github.com/kyri56xcaesar/minioth"
        default creds for admin: 
        - root
        - root

- job scheduling mechanism (modular/configurable)
      - simple queue (default)


- job execution system (modular/configurable)
      - kubernetes
      - docker

- (central) userspace API for accessing + using everything

- frontend application