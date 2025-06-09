import os
import boto3
import ast
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

for line in logic.split("\n"):
    parts = line.strip().lower().split(":")
    if len(parts) != 2:
        
break
    key = parts[0]
    value = parts[1]
    if key == "states":
        STATES = int(value.strip())
    elif key == "generations":
         GENERATIONS = int(value.strip())
    elif key == "neighborhood":
        # parse the value as a rectangular array of arrays
        try:
            arr = ast.literal_eval(value)
            # Ensure it's a rectangular 2D array
            if not (isinstance(arr, list) and all(isinstance(row, list) for row in arr)):
                raise ValueError("NEIGHBORHOOD must be a 2D array")
            row_len = len(arr[0])
            if not all(len(row) == row_len for row in arr):
                raise ValueError("NEIGHBORHOOD array must be rectangular")
            NEIGHBORHOOD = arr
        except Exception as e:
            raise ValueError(f"Failed to parse NEIGHBORHOOD: {e}")
         

print(f"[INFO] STATES: {STATES}")
print(f"[INFO] GENERATIONS: {GENERATIONS}")
print(f"[INFO] NEIGHBORHOOD: {NEIGHBORHOOD}")

# Validate STATES and GENERATIONS: must be positive integers
if  STATES <= 0 or GENERATIONS <= 0:
    raise ValueError("Invalid STATES or GENERATIONS: must be positive non-empty integer values")
if  len(NEIGHBORHOOD) != len(NEIGHBORHOOD[0]):
     raise ValueError("Invalid NEIGHBORHOOD input value")

#######################

# --- Download Input File from MinIO ---
print(f"[INFO] Downloading s3://{input_bucket}/{input_object} to {input_path}")
s3 = boto3.client(
    's3',
    endpoint_url="http://" + minio_endpoint.replace("http://", ""),
    aws_access_key_id=minio_access_key,
    aws_secret_access_key=minio_secret_key,
)
s3.download_file(input_bucket, input_object, input_path)



#############################################
# perform
import numpy as np
from PIL import Image


class CAImageProcessor:
    def __init__(self):
        self.img_palette = None
    def img2array(self, path, states):
        """Converts an image into a numpy array while preserving the palette."""
        img = Image.open(path).convert(mode='P')  # Open image in paletted mode
        self.img_palette = img.getpalette()  # Store palette
        img_arr = np.asarray(img, dtype=np.int32)  # Convert to NumPy array

        # Check the number of unique colors (states) in the image
        total_colors = len(np.unique(img_arr))
        if total_colors > states:
            print("Error: More states in the image than allowed by the rule.")
            
return None

        if np.max(img_arr) > states - 1:
            print("Error: State values out of range. Must be within [0, states-1].")
            
return None

        img.close()
        
return img_arr
    
    def array2img(self, grid_arr_1D, path_out, vertical_offset, total_states):
        """Converts a numpy array back into an image while preserving the original palette."""
        GRID_X = 1920
        GRID_Y = 1080 - vertical_offset

        # Reshape array to 2D
        array_2D = np.reshape(grid_arr_1D, (GRID_Y, GRID_X))
        
        # Convert to image using Palette mode
        img = Image.fromarray(array_2D.astype(np.uint8), mode='P')
        
        # Apply the original palette
        img.putpalette(self.img_palette)
        
        # Save as .bmp file
        img.save(path_out, format='BMP')

# Define transition function based on sum in a 21x21 neighborhood
def apply_transition_rule(grid, neighborhood_size=29, neighborhood=[]):
    NEIGHBORHOOD_WEIGHTS = np.array(neighborhood)
    new_grid = np.zeros_like(grid)
    padding = neighborhood_size // 2
    padded_grid = np.pad(grid, padding, mode='constant', constant_values=0)
    for i in range(grid.shape[0]):
       for j in range(grid.shape[1]):
           sub_matrix = padded_grid[i:i+neighborhood_size, j:j+neighborhood_size]
           neighborhood_sum = np.sum(sub_matrix*NEIGHBORHOOD_WEIGHTS)

           current_state = grid[i, j]
           if current_state == 1 and neighborhood_sum in [0, 1]:
                   new_grid[i, j] = 0
           elif current_state == 1 and neighborhood_sum in [4, 1024]:
                   new_grid[i, j] = 0
           elif current_state == 1 and neighborhood_sum in [2, 3]:
                   new_grid[i, j] = 1
           elif current_state == 0 and neighborhood_sum == 3:
                   new_grid[i, j] = 1
           else:
                   new_grid[i, j] = 0

    
return new_grid

# Initialize the processor
processor = CAImageProcessor()

# Convert the input image to an array
image_path = input_path
output_path = output_path

img_array = processor.img2array(image_path, STATES)

if img_array is not None:
    # Process the CA on the converted image
    for i in range(GENERATIONS):
        img_array = apply_transition_rule(img_array, neighborhood_size=len(NEIGHBORHOOD), neighborhood=NEIGHBORHOOD)

    processor.array2img(img_array.flatten(), output_path, vertical_offset=0, total_states=STATES)
    






















####################################################################



print(f"[INFO] Uploading result to s3://{output_bucket}/{output_object}")
s3.upload_file(output_path, output_bucket, output_object)
print(f"[INFO] Done. File uploaded to s3://{output_bucket}/{output_object}")



