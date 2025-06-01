import os
import boto3
import subprocess
import sys

# --- Environment Variables ---
input_bucket = os.getenv("INPUT_BUCKET", "uspace-default")
input_object = os.getenv("INPUT_OBJECT", "input")
output_bucket = os.getenv("OUTPUT_BUCKET", "uspace-default")
output_object = os.getenv("OUTPUT_OBJECT", "output")
output_format = os.getenv("OUTPUT_FORMAT", "txt")
logic = os.getenv("LOGIC", "cat {input} > {output}")

minio_endpoint = os.getenv("ENDPOINT", "http://minio:9000")
minio_access_key = os.getenv("ACCESS_KEY", "minioadmin")
minio_secret_key = os.getenv("SECRET_KEY", "minioadmin")

input_path = "/tmp/input"
output_path = f"/tmp/output.{output_format}"

# --- Download Input File from MinIO ---
print(f"[INFO] Downloading s3://{input_bucket}/{input_object} to {input_path}")
s3 = boto3.client(
    's3',
    endpoint_url="http://" + minio_endpoint.replace("http://", ""),
    aws_access_key_id=minio_access_key,
    aws_secret_access_key=minio_secret_key,
)
s3.download_file(input_bucket, input_object, input_path)

# --- Replace placeholders in logic ---
shell_command = logic.replace("{input}", input_path).replace("{output}", output_path)

# --- Execute Command ---
print(f"[EXEC] {shell_command}")
result = subprocess.run(shell_command, shell=True, capture_output=True, text=True)

if result.returncode != 0:
    print("[ERROR] bash pipeline failed:\n", result.stderr, file=sys.stderr)
    sys.exit(1)

# --- Upload Output ---
print(f"[INFO] Uploading result to s3://{output_bucket}/{output_object}")
s3.upload_file(output_path, output_bucket, output_object)
print(f"[INFO] Done. File uploaded to s3://{output_bucket}/{output_object}")
