import duckdb
import os
import sqlparse


# get env vars
placeholder = "#%"
query = os.getenv("LOGIC", "SELECT * FROM read_csv_auto('input.csv');")
query = query.replace('"', "'")

input_bucket = os.getenv("INPUT_BUCKET", "uspace-default")
input_object = os.getenv("INPUT_OBJECT", "input.csv")
input_format = os.getenv("INPUT_FORMAT", "csv")

output_bucket = os.getenv("OUTPUT_BUCKET", "uspace-default")
output_object = os.getenv("OUTPUT_OBJECT", "output.csv")
output_format = os.getenv("OUTPUT_FORMAT", "txt")

minio_endpoint = os.getenv("ENDPOINT", "minio:9000")
minio_access_key = os.getenv("ACCESS_KEY", "minioadmin")
minio_secret_key = os.getenv("SECRET_KEY", "minioadmin")

# logs
print(f"[INFO] Starting DuckDB application with query: {query}")
print(f"[INFO] Input bucket: {input_bucket}")
print(f"[INFO] Input object: {input_object}")
print(f"[INFO] Input format: {output_format}")
print(f"[INFO] Output bucket: {output_bucket}")
print(f"[INFO] Output object: {output_object}")
print(f"[INFO] Output format: {output_format}")
print(f"[INFO] MinIO endpoint: {minio_endpoint}")

# should format input query to read from s3://{input_bucket}/{input_object}
if input_format == "csv":
    query = query.replace(placeholder, f"read_csv_auto('s3://{input_bucket}/{input_object}')")
elif input_format == "json":
    query = query.replace(placeholder, f"read_json_auto('s3://{input_bucket}/{input_object}')")
elif input_format == "parquet":
    query = query.replace(placeholder, f"read_parquet('s3://{input_bucket}/{input_object}')")
elif input_format == "txt" or input_format == "text" or input_format == "str":
    query = query.replace(placeholder, f"read_csv_auto('s3://{input_bucket}/{input_object}', delim='\\n', header=False)")
else:
    query = query.replace(placeholder, f"read_csv_auto('s3://{input_bucket}/{input_object}')")


print(f"[INFO] Updated query: {query}")

# init
con = duckdb.connect(database=':memory:')
con.execute("SET enable_http_metadata_cache=false;")
con.execute("INSTALL aws;")
con.execute("LOAD aws;")
con.execute(f"SET s3_region='eu-central-1';")
con.execute(f"SET s3_endpoint='{minio_endpoint}';")
con.execute(f"SET s3_access_key_id='{minio_access_key}';")
con.execute(f"SET s3_secret_access_key='{minio_secret_key}';")
con.execute(f"SET s3_url_style='path';") 
con.execute("SET s3_use_ssl=false;")

# execution
last_stmt = None
for stmt in sqlparse.split(query):
    stmt = stmt.strip()
    if stmt:
        last_stmt = stmt
        print(f"[EXEC] {stmt}")
        con.execute(stmt)

if not last_stmt or not last_stmt.lower().startswith("select"):
    raise ValueError("[ERROR] The last statement is not a SELECT query and cannot be exported.")


# Strip trailing semicolon from last_stmt if present
if last_stmt.endswith(";"):
    last_stmt = last_stmt.rstrip(";")


# output
if output_format == "csv":
    print("about to execute the following...")
    print(f"COPY ({last_stmt}) TO 's3://{output_bucket}/{output_object}' (FORMAT CSV, HEADER true);")
    con.execute(f"COPY ({last_stmt}) TO 's3://{output_bucket}/{output_object}' (FORMAT CSV, HEADER true);")
elif output_format == "json":
    con.execute(f"COPY ({last_stmt}) TO 's3://{output_bucket}/{output_object}' (FORMAT JSON);")
elif output_format == "parquet":
    con.execute(f"COPY ({last_stmt}) TO 's3://{output_bucket}/{output_object}' (FORMAT PARQUET);")
else:
    con.execute(f"COPY ({last_stmt}) TO 's3://{output_bucket}/{output_object}' (FORMAT CSV, HEADER true);")

con.close()
print(f"[INFO] Successfully wrote output to MinIO: s3://{output_bucket}/{output_object}")




