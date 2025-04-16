import duckdb
import sys
import requests
import os

# Environment variables (passed via Kubernetes job)
input_url = os.getenv("INPUT_URL")
query = os.getenv("QUERY", "SELECT * FROM read_csv_auto('input.csv');")
output_path = os.getenv("OUTPUT_PATH", "output.csv")

# Download file from MinIO (or any URL)
input_file = "input.csv"
r = requests.get(input_url)
with open(input_file, 'wb') as f:
    f.write(r.content)

# Run DuckDB
con = duckdb.connect(database=':memory:')
df = con.execute(query).fetchdf()

# Save to CSV
df.to_csv(output_path, index=False)

print(f"[INFO] Wrote result to {output_path}")
