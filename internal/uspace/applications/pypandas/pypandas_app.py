import os
import boto3
import pandas as pd
import numpy as np
import sys
import traceback

# ENV vars
logic_code = os.getenv("LOGIC", "df['result'] = df['value'] * 2")
input_bucket = os.getenv("INPUT_BUCKET", "uspace-default")
input_object = os.getenv("INPUT_OBJECT", "input.csv")
input_format = os.getenv("INPUT_FORMAT", "csv")

output_bucket = os.getenv("OUTPUT_BUCKET", "uspace-default")
output_object = os.getenv("OUTPUT_OBJECT", "output.csv")
output_format = os.getenv("OUTPUT_FORMAT", "csv")

minio_endpoint = os.getenv("ENDPOINT", "http://minio:9000")
minio_access_key = os.getenv("ACCESS_KEY", "minioadmin")
minio_secret_key = os.getenv("SECRET_KEY", "minioadmin")

print(f"[INFO] Input: s3://{input_bucket}/{input_object}")
print(f"[INFO] Output: s3://{output_bucket}/{output_object}")
print(f"[INFO] Executing logic:\n{logic_code}")

# Setup boto3 MinIO client
s3 = boto3.client(
    's3',
    endpoint_url="http://" + minio_endpoint.replace("http://", ""),
    aws_access_key_id=minio_access_key,
    aws_secret_access_key=minio_secret_key,
    region_name="eu-central-1"
)

# Download file from MinIO
input_tmp_path = "/tmp/input"
s3.download_file(input_bucket, input_object, input_tmp_path)

# Load into DataFrame
if input_format == "csv":
    df = pd.read_csv(input_tmp_path)
elif input_format == "json":
    df = pd.read_json(input_tmp_path)
elif input_format == "parquet":
    df = pd.read_parquet(input_tmp_path)
else:
    raise ValueError(f"Unsupported input format: {input_format}")

# Execute the logic in a safe namespace
try:
    exec(logic_code, {"pd": pd, "np": np}, {"df": df})
except Exception as e:
    print("[ERROR] Error during logic execution:")
    traceback.print_exc()
    sys.exit(1)

# Save output
output_tmp_path = "/tmp/output"
if output_format == "csv":
    df.to_csv(output_tmp_path, index=False)
elif output_format == "json":
    df.to_json(output_tmp_path)
elif output_format == "parquet":
    df.to_parquet(output_tmp_path)
else:
    raise ValueError(f"Unsupported output format: {output_format}")

# Upload back to MinIO
s3.upload_file(output_tmp_path, output_bucket, output_object)
print(f"[INFO] Result written to s3://{output_bucket}/{output_object}")
