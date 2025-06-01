import os
import boto3
import subprocess
import sys

input_bucket = os.getenv("INPUT_BUCKET", "uspace-default")
input_object = os.getenv("INPUT_OBJECT", "input.csv")
input_format = os.getenv("INPUT_FORMAT", "csv")

output_bucket = os.getenv("OUTPUT_BUCKET", "uspace-default")
output_object = os.getenv("OUTPUT_OBJECT", "output.csv")
output_format = os.getenv("OUTPUT_FORMAT", "csv")

minio_endpoint = os.getenv("ENDPOINT", "http://minio:9000")
minio_access_key = os.getenv("ACCESS_KEY", "minioadmin")
minio_secret_key = os.getenv("SECRET_KEY", "minioadmin")

logic_code = os.getenv("LOGIC", "output = input .* 2;")

print(f"[INFO] Running Octave job: {logic_code}")

# Download input from MinIO
s3 = boto3.client(
    's3',
    endpoint_url="http://" + minio_endpoint.replace("http://", ""),
    aws_access_key_id=minio_access_key,
    aws_secret_access_key=minio_secret_key,
)

s3.download_file(input_bucket, input_object, "/tmp/input.csv")

# Write Octave script to disk
octave_script = f"""
input = csvread('/tmp/input.csv');
{logic_code}
csvwrite('/tmp/output.csv', output);
"""

with open("/tmp/script.m", "w") as f:
    f.write(octave_script)

# Execute Octave
print("[INFO] Executing Octave script...")
result = subprocess.run(["octave", "--quiet", "--no-window-system", "/tmp/script.m"], capture_output=True, text=True)
print(result.stdout)
if result.returncode != 0:
    print(result.stderr, file=sys.stderr)
    raise RuntimeError("Octave execution failed")

# Upload result
s3.upload_file("/tmp/output.csv", output_bucket, output_object)
print(f"[INFO] Result written to MinIO: s3://{output_bucket}/{output_object}")
