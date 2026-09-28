#!/usr/bin/env bash
# kuspace end-to-end smoke test.
#
# Drives a running stack through frontapp the way a browser does: accounts,
# auth rejection, admin, storage round trip, cross-user isolation, jobs
# authorization, dashboard/system pages, security headers, and a scan of
# every container log for leaked secrets.
#
# usage: scripts/smoke.sh            (stack from deployments/docker-compose)
#   FRONT=http://host:port  AUTH=http://host:port  WSS=http://host:port
#   SECRETS=path/to/secrets.env  COMPOSE_DIR=path   override the defaults
#   SMOKE_JOBS=0   skip running real jobs (needs a job executor, e.g. docker)
# exit status: number of failed checks (0 = all passed)

set -u
ROOT=$(cd "$(dirname "$0")/.." && pwd)
FRONT=${FRONT:-http://localhost:18080}; F=$FRONT/api/v1
AUTH=${AUTH:-http://localhost:9090}; M=$AUTH/v1
WSS=${WSS:-http://localhost:18082}
USPACE=${USPACE:-http://localhost:18079}
SECRETS=${SECRETS:-$ROOT/configs/secrets.env}
COMPOSE_DIR=${COMPOSE_DIR:-$ROOT/deployments/docker-compose}

[ -r "$SECRETS" ] || { echo "cannot read $SECRETS (see configs/secrets.env.example)"; exit 1; }
J=$(mktemp -d); trap 'rm -rf "$J"' EXIT
ROOTPW=$(sed -n 's/^MINIOTH_SECRET_KEY=//p' "$SECRETS")
SVC=$(sed -n 's/^SERVICE_SECRET_KEY=//p' "$SECRETS")
# every secret value (service-map entries split out); the public MinIO
# default "minioadmin" is also a username, so it can't count as a leak
grep -v '^#' "$SECRETS" | cut -d= -f2- | tr ',' '\n' | sed 's/^.*://' |
  grep -v -x -e '' -e minioadmin | sort -u > "$J/secrets"

# CSRF (double-submit): every jar carries a known csrf_token and every curl
# call sends it back as X-Csrf-Token, like csrf.js does in the browser
CSRF=$(printf 'ab%.0s' $(seq 32))
export CURL_HOME=$J
printf 'header = "X-Csrf-Token: %s"\n' "$CSRF" > "$J/.curlrc"
FRONT_HOST=$(python3 -c "import urllib.parse,sys; print(urllib.parse.urlparse(sys.argv[1]).hostname)" "$FRONT")
fixjar() { # set the jar's csrf_token to $CSRF
  sed -i '/\tcsrf_token\t/d' "$1" 2>/dev/null
  printf '%s\tFALSE\t/\tFALSE\t0\tcsrf_token\t%s\n' "$FRONT_HOST" "$CSRF" >> "$1"
}

U=smoke$RANDOM; PASS=0; FAIL=0
check() { # name expected(glob) actual
  if [[ "$3" == $2 ]]; then echo "PASS  $1 ($3)"; PASS=$((PASS + 1))
  else echo "FAIL  $1: expected $2, got $3"; FAIL=$((FAIL + 1)); fi
}
yes_if() { if "$@"; then echo yes; else echo no; fi; }
code() { curl -s -o "$J/body" -w '%{http_code}' "$@"; }
json() { python3 -c "import json,sys; d=json.load(sys.stdin); $1"; }
register() { curl -s -o /dev/null -X POST "$F/register" -d "username=$1&password=smokepass123&repeatPassword=smokepass123${2:-}"; }
login() { curl -s -o /dev/null -c "$J/$1.jar" -X POST "$F/login" -d "username=$1&password=smokepass123"; fixjar "$J/$1.jar"; }

echo "--- health"
check "minioth well-known"          200 "$(code "$M/.well-known/minioth")"
check "frontapp login page"         200 "$(code "$F/login")"
check "wss healthz"                 200 "$(code "$WSS/healthz")"

echo "--- accounts"
check "register -> redirect"        303 "$(code -X POST "$F/register" -d "username=$U&password=smokepass123&repeatPassword=smokepass123&email=$U@test.local")"
check "register duplicate refused"  "4??" "$(code -X POST "$F/register" -d "username=$U&password=smokepass123&repeatPassword=smokepass123")"
check "wrong password refused"      "4??" "$(code -X POST "$F/login" -d "username=$U&password=wrongpass999")"
check "login -> redirect"           303 "$(code -D "$J/login.hdr" -c "$J/user.jar" -X POST "$F/login" -d "username=$U&password=smokepass123")"
check "session cookie set"          yes "$(yes_if grep -q accessToken "$J/user.jar")"
fixjar "$J/user.jar"
check "cookie is SameSite=Strict"   yes "$(yes_if grep -qi 'set-cookie: accessToken=.*samesite=strict' "$J/login.hdr")"
check "panel for user"              200 "$(code -b "$J/user.jar" "$F/verified/admin-panel")"
check "panel shows username"        yes "$(yes_if grep -q "$U" "$J/body")"
check "user blocked from admin API" "40?" "$(code -b "$J/user.jar" "$F/verified/admin/fetch-users?format=json")"
check "email update"                200 "$(code -b "$J/user.jar" -X PUT "$F/verified/user-update" -d "new-email-change=new_$U@test.local")"

echo "--- auth rejection"
check "no cookie"                   "40?" "$(code "$F/verified/admin-panel")"
check "garbage token"               401 "$(code -b 'accessToken=not.a.jwt' "$F/verified/admin-panel")"
FORGED=$(printf '%s' '{"alg":"none"}' | base64 -w0 | tr -d =).$(printf '%s' '{"username":"x","groups":"admin"}' | base64 -w0 | tr -d =).
check "alg=none token"              401 "$(code -b "accessToken=$FORGED" "$F/verified/admin/fetch-users")"

echo "--- security headers"
curl -s -D "$J/hdr" -o /dev/null "$F/login"
check "Content-Security-Policy"     yes "$(yes_if grep -qi '^content-security-policy:' "$J/hdr")"
check "X-Frame-Options DENY"        yes "$(yes_if grep -qi '^x-frame-options: deny' "$J/hdr")"
check "no HSTS over plain http"     no  "$(yes_if grep -qi '^strict-transport-security:' "$J/hdr")"

echo "--- admin"
check "admin login"                 303 "$(code -c "$J/admin.jar" -X POST "$F/login" -d "username=kuspaceadmin&password=$ROOTPW")"
fixjar "$J/admin.jar"
check "admin lists users"           200 "$(code -b "$J/admin.jar" "$F/verified/admin/fetch-users?format=json")"
check "new user listed"             yes "$(yes_if grep -q "$U" "$J/body")"
check "no bcrypt hash in response"  no  "$(yes_if grep -q '\$2[aby]\$' "$J/body")"
check "email update persisted"      yes "$(yes_if grep -q "new_$U@test.local" "$J/body")"
check "admin lists groups"          200 "$(code -b "$J/admin.jar" "$F/verified/admin/fetch-groups?format=json")"
check "admin system-conf"           200 "$(code -b "$J/admin.jar" "$F/verified/admin/system-conf?format=json")"
check "system-conf leaks no secret" no  "$(yes_if grep -q -F -f "$J/secrets" "$J/body")"
check "admin lists volumes"         200 "$(code -b "$J/admin.jar" "$F/verified/fetch-volumes")"

echo "--- storage round trip"
FN=hello_$U.txt; echo "hello kuspace $U" > "$J/$FN"
check "upload"                      "20?" "$(code -b "$J/user.jar" -F "files=@$J/$FN" "$F/verified/upload")"
check "empty upload refused"        "4??" "$(code -b "$J/user.jar" -F "other=x" "$F/verified/upload")"
check "list own files"              200 "$(code -b "$J/user.jar" "$F/verified/fetch-resources?format=json")"
check "uploaded file listed"        yes "$(yes_if grep -q "$FN" "$J/body")"
RID=$(json "print(next(r['rid'] for r in d if r['name']=='/$FN'))" < "$J/body")
check "download"                    200 "$(code -b "$J/user.jar" "$F/verified/download?target=/$FN&volume=uspace-default")"
check "downloaded content matches"  yes "$(yes_if grep -q "hello kuspace $U" "$J/body")"
check "chmod to private"            200 "$(code -b "$J/user.jar" -X PATCH "$F/verified/admin/chmod?rid=$RID" -d 'permissions=rw-------')"

echo "--- cross-user isolation"
O=${U}o; register "$O"; login "$O"
check "private file hidden from others" no "$(curl -s -b "$J/$O.jar" "$F/verified/fetch-resources?format=json" | yes_if grep -q "\"/$FN\"")"
check "other user download refused" "40?" "$(code -b "$J/$O.jar" "$F/verified/download?target=/$FN&volume=uspace-default")"
check "other user delete refused"   403 "$(code -b "$J/$O.jar" -X DELETE "$F/verified/rm?name=/$FN")"
check "forged root header: delete"  403 "$(code -b "$J/$O.jar" -H "Access-Target: :uspace-default:/$FN 0:0" -X DELETE "$F/verified/rm?name=/$FN")"
check "forged root header: download" "40?" "$(code -b "$J/$O.jar" -H "Access-Target: :uspace-default:/$FN 0:0" "$F/verified/download?target=/$FN&volume=uspace-default")"
mkdir -p "$J/o" && echo "overwritten by $O" > "$J/o/$FN"
check "same-name upload refused"    409 "$(code -b "$J/$O.jar" -F "files=@$J/o/$FN" "$F/verified/upload")"
OWN=o_$U.txt; echo "mine" > "$J/$OWN"; curl -s -o /dev/null -b "$J/$O.jar" -F "files=@$J/$OWN" "$F/verified/upload"
check "copy onto other's file refused" 409 "$(code -b "$J/$O.jar" -X POST "$F/verified/cp?resource=/$OWN&dest=uspace-default/$FN")"
check "owner's content intact"      yes "$(curl -s -b "$J/user.jar" "$F/verified/download?target=/$FN&volume=uspace-default" | yes_if grep -q "hello kuspace $U")"

echo "--- identity and request forgery"
check "identity smuggled in a target" "40?" "$(code -b "$J/$O.jar" "$F/verified/download?target=/$FN%200:0&volume=uspace-default")"
check "write without CSRF token"    403 "$(curl -q -s -o "$J/body" -w '%{http_code}' -b "$J/$O.jar" -X DELETE "$F/verified/rm?name=/$OWN")"
check "cross-site write refused"    403 "$(code -b "$J/$O.jar" -H 'Origin: https://evil.example' -X DELETE "$F/verified/rm?name=/$OWN")"

echo "--- group volumes"
GV=team$(echo "$U" | tr -dc 'a-z0-9')
check "admin creates a volume"      "20?" "$(code -b "$J/admin.jar" -H 'Content-Type: application/json' -X POST "$F/verified/admin/volumeadd" -d "{\"name\":\"$GV\",\"capacity\":1}")"
# the user's own group (minioth's /admin/users reports pgroup = uid; the token and the group list agree on the real gid)
PG=$(curl -s -b "$J/admin.jar" "$F/verified/admin/fetch-users?format=json" | json "print(next(g['gid'] for u in d if u['username']=='$U' for g in u.get('groups') or [] if g['groupname']=='$U'))")
check "volume given to the user's group" 200 "$(code -X POST "$USPACE/api/v1/admin/group/volume" -H "X-Service-Secret: $SVC" -H 'Access-Target: 0::/ 0:0' -H 'Content-Type: application/json' -d "{\"vname\":\"$GV\",\"gid\":${PG:-0},\"quota\":1}")"
check "member sees the shared volume" yes "$(curl -s -b "$J/user.jar" "$F/verified/fetch-volumes?format=json" | yes_if grep -q "\"$GV\"")"
check "member uploads to it"        "20?" "$(code -b "$J/user.jar" -H "X-Volume-Target: $GV" -F "files=@$J/$FN" "$F/verified/upload")"
check "non-member upload refused"   403 "$(code -b "$J/$O.jar" -H "X-Volume-Target: $GV" -F "files=@$J/$OWN" "$F/verified/upload")"
check "file belongs to the group"   "${PG:-none}" "$(curl -s -b "$J/user.jar" "$F/verified/fetch-resources?format=json&volume=$GV" | json "print(next((r.get('gid','') for r in d if r['name']=='/$FN'),''))")"
check "member deletes the file"     200 "$(code -b "$J/user.jar" -X DELETE "$F/verified/rm?name=/$FN&volume=$GV")"
check "admin deletes the volume"    "20?" "$(code -b "$J/admin.jar" -X DELETE "$F/verified/admin/volumedel?volume=$GV")"

echo "--- jobs authorization"
job() { code -b "$J/$1.jar" -X POST "$F/verified/jobs" --data-urlencode "input=uspace-default/$2" \
  --data-urlencode "output=uspace-default/out_$1_$RANDOM.csv" --data-urlencode "logic=duckdb" \
  --data-urlencode "logicBody=SELECT * FROM {input};" --data-urlencode "parallelism=1"; }
check "job on own file accepted"    200 "$(job user "$FN")"
JID=$(json 'print(d.get("jid",""))' < "$J/body")
check "job on other's private file refused" 403 "$(job "$O" "$FN")"
check "others don't see my jobs"    no  "$(curl -s -b "$J/$O.jar" "$F/verified/fetch-jobs?format=json" | yes_if grep -q "/$FN\"")"

echo "--- live job output (wss tickets)"
ws() { curl -s -o /dev/null -w '%{http_code}' --http1.1 -m 3 -H 'Connection: Upgrade' -H 'Upgrade: websocket' \
  -H 'Sec-WebSocket-Version: 13' -H 'Sec-WebSocket-Key: c21va2V0ZXN0a2V5MTIzNA==' "$@"; }
check "consumer without ticket refused" 401 "$(ws "$WSS/get-session?jid=$JID&role=consumer")"
check "producer without service secret refused" 401 "$(ws "$WSS/get-session?jid=$JID&role=producer")"
check "owner gets a ticket"         200 "$(code -b "$J/user.jar" "$F/verified/ws-ticket?jid=$JID")"
TICKET=$(json 'print(d["ticket"])' < "$J/body")
check "ticket opens the stream"     101 "$(ws "$WSS/get-session?jid=$JID&role=consumer&ticket=$TICKET")"
check "ticket is for this job only" 401 "$(ws "$WSS/get-session?jid=$((JID + 1000))&role=consumer&ticket=$TICKET")"
check "no ticket for others' jobs"  403 "$(code -b "$J/$O.jar" "$F/verified/ws-ticket?jid=$JID")"
check "owner reads the job log"     200 "$(code -b "$J/user.jar" "$F/verified/job-log?jid=$JID")"
check "others can't read the job log" 403 "$(code -b "$J/$O.jar" "$F/verified/job-log?jid=$JID")"
check "wss session delete is service-only" 401 "$(code -X DELETE "$WSS/delete-session?jid=$JID")"

echo "--- uspace used directly with a minioth token (no frontapp)"
TOKEN=$(curl -s -X POST "$M/login" -H 'Content-Type: application/json' -d "{\"username\":\"$U\",\"password\":\"smokepass123\"}" | json 'print(d.get("access_token",""))')
check "minioth issues an access token" yes "$(yes_if test -n "$TOKEN")"
check "no credentials refused"      401 "$(code "$USPACE/api/v1/resources" -H 'Access-Target: :uspace-default:/')"
check "token lists own files"       200 "$(code -H "Authorization: Bearer $TOKEN" -H 'Access-Target: :uspace-default:/' "$USPACE/api/v1/resources")"
check "own file visible"            yes "$(yes_if grep -q "$FN" "$J/body")"
check "forged root identity ignored" no "$(code -H "Authorization: Bearer $TOKEN" -H 'Access-Target: :uspace-default:/ 0:0' "$USPACE/api/v1/resources" >/dev/null; yes_if grep -q "\"/o_$U.txt\"" "$J/body")"
check "token can't reach admin API" 403 "$(code -H "Authorization: Bearer $TOKEN" -H 'Access-Target: 0::/ 0:0' "$USPACE/api/v1/admin/volumes")"

echo "--- apps, dashboard, system"
curl -s -b "$J/user.jar" "$F/verified/fetch-apps?format=json" > "$J/apps"
check "default apps installed"      6 "$(json 'd=d.get("content",d) if isinstance(d,dict) else d; print(len(d))' < "$J/apps")"
check "bash image name valid"       "*:applications-bash-v2" "$(json 'd=d.get("content",d) if isinstance(d,dict) else d; print(next(a["image"] for a in d if a["name"]=="bash"))' < "$J/apps")"
check "all services up"             "4/4" "$(curl -s -b "$J/admin.jar" "$F/verified/admin/system-status?format=json" | json 'print(str(sum(s["up"] for s in d))+"/"+str(len(d)))')"
check "dashboard served"            yes "$(curl -s -b "$J/user.jar" "$F/verified/admin-panel" | yes_if grep -q 'id="dash"')"
check "user blocked from system page" "40?" "$(code -b "$J/user.jar" "$F/verified/admin/system-status")"

if [ "${SMOKE_JOBS:-1}" != "0" ]; then
  echo "--- jobs actually run (executor)"
  printf 'name,score\nann,3\nbob,7\ncid,5\n' > "$J/in_$U.csv"
  curl -s -o /dev/null -b "$J/user.jar" -F "files=@$J/in_$U.csv" "$F/verified/upload"
  run_job() { curl -s -b "$J/user.jar" -X POST "$F/verified/jobs" --data-urlencode "input=uspace-default/in_$U.csv" \
    --data-urlencode "output=uspace-default/$1" --data-urlencode "logic=bash" --data-urlencode "logicBody=$2" \
    --data-urlencode "parallelism=1" --data-urlencode "timeout=5" | json 'print(d.get("jid",""))'; }
  job_field() { curl -s -b "$J/user.jar" "$F/verified/fetch-jobs?format=json" | json "j=[x for x in d['content'] if x['jid']==$1]; print(j[0].get('$2','') if j else '')"; }
  wait_job() { local s; for _ in $(seq 1 90); do s=$(job_field "$1" status); [[ " $2 " == *" $s "* ]] && { echo "$s"; return; }; sleep 1; done; echo "timeout($s)"; }
  RJ=$(run_job "sorted_$U.csv" 'sort -t, -k2 -nr {input} > {output}')
  check "job completed"               completed "$(wait_job "$RJ" 'completed failed cancelled')"
  check "engine recorded"             "?*" "$(job_field "$RJ" engine)"
  check "job output content"          "bob,7" "$(curl -s -b "$J/user.jar" "$F/verified/download?target=/sorted_$U.csv&volume=uspace-default" | head -1)"
  check "job log saved"               yes "$(curl -s -b "$J/user.jar" "$F/verified/job-log?jid=$RJ" | yes_if grep -q 'finished with status: completed')"
  res_field() { curl -s -b "$J/user.jar" "$F/verified/fetch-resources?format=json" | json "d=d.get('content',d) if isinstance(d,dict) else d; r=[x for x in d if x.get('name','').lstrip('/')=='$1']; print(r[0].get('$2','') if r else '')"; }
  IG=$(res_field "in_$U.csv" gid)
  check "job output gets the owner's primary group" "${IG:-no-input-gid}" "$(res_field "sorted_$U.csv" gid)"
  # code runs from $LOGIC, never pasted into the shell: quotes and $ survive
  CJ=$(curl -s -b "$J/user.jar" -X POST "$F/verified/jobs" --data-urlencode "input=uspace-default/in_$U.csv" \
    --data-urlencode "output=uspace-default/code_$U.txt" --data-urlencode "logic=py:3.12-alpine" \
    --data-urlencode 'logicBody=print("it'"'"'s \"quoted\" $HOME")' \
    --data-urlencode "parallelism=1" --data-urlencode "timeout=5" | json 'print(d.get("jid",""))')
  check "python code job completed"   completed "$(wait_job "$CJ" 'completed failed cancelled')"
  check "code with quotes ran verbatim" yes "$(curl -s -b "$J/user.jar" "$F/verified/job-log?jid=$CJ" | yes_if grep -qF "it's \"quoted\" \$HOME")"
  SJ=$(run_job "slow_$U.csv" 'sleep 60; cp {input} {output}')
  check "slow job running"            running "$(wait_job "$SJ" 'running completed failed')"
  check "cancel accepted"             200 "$(code -b "$J/user.jar" -X POST "$F/verified/job-cancel?jid=$SJ")"
  check "job cancelled"               cancelled "$(wait_job "$SJ" 'cancelled completed failed')"
  check "others can't cancel my jobs" 403 "$(code -b "$J/$O.jar" -X POST "$F/verified/job-cancel?jid=$RJ")"
  check "finished job not cancellable" 409 "$(code -b "$J/user.jar" -X POST "$F/verified/job-cancel?jid=$RJ")"
fi

echo "--- ending sessions (last: they revoke the user's tokens)"
check "password change"             204 "$(code -b "$J/user.jar" -X POST "$F/verified/passwd" -d "currentPassword=smokepass123&newPassword=smokepass456&newPasswordRepeat=smokepass456")"
check "old session revoked"         401 "$(code -b "$J/user.jar" "$F/verified/admin-panel")"
check "old password refused"        "4??" "$(code -X POST "$F/login" -d "username=$U&password=smokepass123")"
check "login with new password"     303 "$(code -c "$J/new.jar" -X POST "$F/login" -d "username=$U&password=smokepass456")"
fixjar "$J/new.jar"
check "logout"                      204 "$(code -b "$J/new.jar" -X DELETE "$F/logout")"
# -b alone doesn't update the jar: it still holds the token logout revoked
check "logged-out token revoked"    401 "$(code -b "$J/new.jar" "$F/verified/admin-panel")"

if [ -d "$COMPOSE_DIR" ] && command -v docker >/dev/null; then
  echo "--- secret hygiene"
  check "no secret in any container log" no "$( (cd "$COMPOSE_DIR" && docker compose logs --no-color 2>&1) | yes_if grep -q -F -f "$J/secrets")"
fi

echo; echo "RESULT: $PASS passed, $FAIL failed"
exit "$FAIL"
