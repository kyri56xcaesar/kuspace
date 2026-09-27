from kuspace_io import fetch_input, put_output
import os
import subprocess
import sys

input_bucket = os.getenv("INPUT_BUCKET", "uspace-default")
input_object = os.getenv("INPUT_OBJECT", "input.csv")
input_format = os.getenv("INPUT_FORMAT", "csv")

output_bucket = os.getenv("OUTPUT_BUCKET", "uspace-default")
output_object = os.getenv("OUTPUT_OBJECT", "output.csv")
output_format = os.getenv("OUTPUT_FORMAT", "csv")


logic_code = os.getenv("LOGIC", "output = input .* 2;")

print(f"[INFO] Running Octave job: {logic_code}")

# Download the input

fetch_input("/tmp/input.csv")

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
put_output("/tmp/output.csv")
print(f"[INFO] Result written to MinIO: s3://{output_bucket}/{output_object}")
