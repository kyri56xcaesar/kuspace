# minioth

## an **_Auth_** service aspiring to become an identity provider

> - stores users/groups
>
> - bcrypt hashed passwords
>
> - JWT - RS256/HS256, (choose by setting header: X-Auth-Signing-Alg)
>
> - storage handler
>>
>> 1. local database sqlite3/duckdb  
>> or  
>> 2. txt files: passwd/groups/shadow (unix style)

### **! can be used serverless, to connect to a different service !**
