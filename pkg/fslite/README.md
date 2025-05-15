# fslite

## Manage (small) volumes locally to save files

fslite can function as a storage system or a serverless user file provisioner

> file/directory storage on a local fs (inspired by object/buckets [minio])

- http served, user provisioned
metadata of contents on a local database sqlite3/duckdb
- can create/delete/limit a volume

### **! can be used serverless, to connect to a different service !**

[config]
set **fsl_locality** [bool] (to save files locally)
set **fsl_server** [bool] (to have a server)
