from kuspace_io import fetch_input, put_output
import os
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


print(f"[INFO] Input: s3://{input_bucket}/{input_object}")
print(f"[INFO] Output: s3://{output_bucket}/{output_object}")
print(f"[INFO] Executing logic:\n{logic_code}")


# Download the input
input_tmp_path = "/tmp/input"
fetch_input(input_tmp_path)

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

# Upload the result
put_output(output_tmp_path)
print(f"[INFO] Result written to s3://{output_bucket}/{output_object}")
