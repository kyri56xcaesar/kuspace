import duckdb
import sys
import requests
import os

# Environment variables (passed via Kubernetes job)
input_url = os.getenv("INPUT")
query = os.getenv("LOGIC", "SELECT * FROM read_csv_auto('input.csv');")
output_path = os.getenv("OUTPUT", "output.csv")
output_format = os.getenv("OUTPUT_FORMAT", "csv")

if input_url == "":
    sys.exit()

# Download file from MinIO (or any URL)
input_file = "input.csv"
r = requests.get(input_url)
with open(input_file, 'wb') as f:
    f.write(r.content)

# Run DuckDB
con = duckdb.connect(database=':memory:')
df = con.execute(query).fetchdf()

# Save to CSV
if output_format == "csv":
    df.to_csv(output_path, index=False)
elif output_format == "json":
    df.to_json(output_path, index=False)
else:
    df.to_csv(output_path, index=False)

print(f"[INFO] Wrote result to {output_path} in {output_format} format")
