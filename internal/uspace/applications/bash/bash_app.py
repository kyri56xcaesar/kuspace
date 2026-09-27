from kuspace_io import fetch_input, put_output
import os
import subprocess
import sys

# --- Environment Variables ---
input_bucket = os.getenv("INPUT_BUCKET", "uspace-default")
input_object = os.getenv("INPUT_OBJECT", "input")
output_bucket = os.getenv("OUTPUT_BUCKET", "uspace-default")
output_object = os.getenv("OUTPUT_OBJECT", "output")
output_format = os.getenv("OUTPUT_FORMAT", "txt")
logic = os.getenv("LOGIC", "cat {input} > {output}")


input_path = "/tmp/input"
output_path = f"/tmp/output.{output_format}"

# --- Download the input ---
print(f"[INFO] Downloading s3://{input_bucket}/{input_object} to {input_path}")
fetch_input(input_path)

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
put_output(output_path)
print(f"[INFO] Done. File uploaded to s3://{output_bucket}/{output_object}")
