"""Job input/output for kuspace applications.

uspace hands every job two presigned URLs instead of storage credentials:

    INPUT_URL   GET  -> the job's input object
    OUTPUT_URL  PUT  -> where the job's result goes

They grant access to exactly those two objects, only while the job may
run. Standard library only.
"""

import os
import shutil
import sys
import urllib.error
import urllib.request

CHUNK = 1 << 20


def _url(name):
    url = os.getenv(name)
    if not url:
        print(f"[ERROR] {name} is not set (uspace provides it)", file=sys.stderr)
        sys.exit(2)
    return url


def fetch_input(path):
    """Download the job's input to path."""
    print(f"[INFO] Downloading input to {path}")
    try:
        with urllib.request.urlopen(_url("INPUT_URL"), timeout=60) as resp, open(path, "wb") as out:
            shutil.copyfileobj(resp, out, CHUNK)
    except urllib.error.HTTPError as e:
        print(f"[ERROR] input download failed: HTTP {e.code} {e.reason}", file=sys.stderr)
        sys.exit(1)
    return path


def put_output(path):
    """Upload path as the job's output."""
    size = os.path.getsize(path)
    print(f"[INFO] Uploading output ({size} bytes)")
    with open(path, "rb") as body:
        req = urllib.request.Request(_url("OUTPUT_URL"), data=body, method="PUT",
                                     headers={"Content-Length": str(size)})
        try:
            with urllib.request.urlopen(req, timeout=300) as resp:
                resp.read()
        except urllib.error.HTTPError as e:
            print(f"[ERROR] output upload failed: HTTP {e.code} {e.reason}", file=sys.stderr)
            sys.exit(1)
    print("[INFO] Output uploaded")
