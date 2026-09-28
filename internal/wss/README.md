# wss

The WebSocket relay for live job output. uspace **produces** a job's output lines,
and browsers **consume** them. wss keeps one session per job id, fans each message
out to that job's consumers, and forgets the session when the job ends. It stores
nothing: the saved log comes from uspace (`GET /job/log`). Entry point: `cmd/wss`,
config `configs/wss.conf`.

## Flow

```
browser ── GET /verified/ws-ticket?jid=7 ──► frontapp    (checks the job is the user's)
browser ◄── ticket (HMAC, 1 min, bound to jid + role) ──┘
browser ── GET /get-session?jid=7&role=consumer&ticket=… ──► wss   (upgrade)
uspace  ── GET /get-session?jid=7&role=producer  + X-Service-Secret ──► wss
uspace  ── DELETE /delete-session?jid=7          + X-Service-Secret ──► wss   (job done)
```

## Endpoints

| Route | Who |
|---|---|
| `GET /get-session?jid=&role=producer` | services: `X-Service-Secret` |
| `GET /get-session?jid=&role=consumer&ticket=` | anyone holding a ticket for this job (`utils.SignWSTicket` / `VerifyWSTicket`, keyed from `SERVICE_SECRET_KEY`) |
| `DELETE /delete-session?jid=` | services |
| `GET /system-conf` | services (secrets never included) |
| `GET /healthz` | anyone |

Browser upgrades are accepted only from the same host name as wss (frontapp and wss
differ only in port). Clients without an `Origin` header, such as uspace, are accepted.

## Configuration

| Variable | Default | |
|---|---|---|
| `J_WS_ADDRESS` | `localhost:8082` | listen address |
| `J_WS_LOGS_PATH` | `data/logs/jobs/` | directory for the per-session logs (`ws-server-<jid>.log`) |
| `SERVICE_SECRET_KEY` | required | producer/admin auth and the ticket key |

## Limits

- Sessions live in process memory, so there is one wss replica. Scaling out would
  need sticky routing by job id or a shared broker.
- A consumer that falls 256 messages behind is disconnected rather than blocking the
  job; the saved log in uspace is complete.
