from kuspace_io import fetch_input, put_output
import duckdb
import os
import sqlparse


# get env vars
placeholder = "{input}"
query = os.getenv("LOGIC", "SELECT * FROM read_csv_auto('input.csv');")
query = query.replace('"', "'")

input_bucket = os.getenv("INPUT_BUCKET", "uspace-default")
input_object = os.getenv("INPUT_OBJECT", "input.csv")
input_format = os.getenv("INPUT_FORMAT", "csv")

output_bucket = os.getenv("OUTPUT_BUCKET", "uspace-default")
output_object = os.getenv("OUTPUT_OBJECT", "output.csv")
output_format = os.getenv("OUTPUT_FORMAT", "txt")

input_path = f"/tmp/input.{input_format}"
output_path = f"/tmp/output.{output_format}"

# logs
print(f"[INFO] Starting DuckDB application with query: {query}")
print(f"[INFO] Input: {input_bucket}/{input_object} ({input_format})")
print(f"[INFO] Output: {output_bucket}/{output_object} ({output_format})")

# the query reads a local copy of the input: no storage credentials needed
fetch_input(input_path)
readers = {
    "csv": f"read_csv_auto('{input_path}')",
    "json": f"read_json_auto('{input_path}')",
    "parquet": f"read_parquet('{input_path}')",
    "txt": f"read_csv_auto('{input_path}', delim='\\n', header=False)",
}
readers["text"] = readers["str"] = readers["txt"]
query = query.replace(placeholder, readers.get(input_format, readers["csv"]))

print(f"[INFO] Updated query: {query}")

# init
con = duckdb.connect(database=':memory:')

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
formats = {"csv": "FORMAT CSV, HEADER true", "json": "FORMAT JSON", "parquet": "FORMAT PARQUET"}
copy = f"COPY ({last_stmt}) TO '{output_path}' ({formats.get(output_format, formats['csv'])});"
print(f"[EXEC] {copy}")
con.execute(copy)
con.close()

put_output(output_path)
print(f"[INFO] Successfully wrote output to {output_bucket}/{output_object}")
