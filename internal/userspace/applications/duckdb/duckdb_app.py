import duckdb
import os


# get env vars
query = os.getenv("LOGIC", "SELECT * FROM read_csv_auto('input.csv');")
output_path = os.getenv("OUTPUT", "output.csv")
minio_bucket = os.getenv("DEFAULT_V", "uspace-default")
output_format = os.getenv("OUTPUT_FORMAT", "csv")

minio_endpoint = os.getenv("ENDPOINT", "minio:9000")
minio_access_key = os.getenv("ACCESS_KEY", "minioadmin")
minio_secret_key = os.getenv("SECRET_KEY", "minioadmin")

print(f"[INFO] Starting DuckDB application with query: {query}")
print(f"[INFO] Output bucket: {minio_bucket}")
print(f"[INFO] Output path: {output_path}")
print(f"[INFO] Output format: {output_format}")
print(f"[INFO] MinIO endpoint: {minio_endpoint}")

           
con = duckdb.connect(database=':memory:')
con.execute("INSTALL aws;")
con.execute("LOAD aws;")

con.execute(f"SET s3_region='us-east-1';")
con.execute(f"SET s3_endpoint='{minio_endpoint}';")
con.execute(f"SET s3_access_key_id='{minio_access_key}';")
con.execute(f"SET s3_secret_access_key='{minio_secret_key}';")
con.execute(f"SET s3_url_style='path';") 

if output_format == "csv":
    con.execute(f"COPY ({query}) TO 's3://{minio_bucket}/{output_path}' (FORMAT CSV, HEADER true);")
elif output_format == "json":
    con.execute(f"COPY ({query}) TO 's3://{minio_bucket}/{output_path}' (FORMAT JSON);")
elif output_format == "parquet":
    con.execute(f"COPY ({query}) TO 's3://{minio_bucket}/{output_path}' (FORMAT PARQUET);")
else:
    con.execute(f"COPY ({query}) TO 's3://{minio_bucket}/{output_path}' (FORMAT CSV, HEADER true);")

con.close()
print(f"[INFO] Successfully wrote output to MinIO: s3://{minio_bucket}/{output_path}")




