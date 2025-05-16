import pandas as pd

# Sample data
df = pd.DataFrame({
    'a': ['x', 'y', 'x', 'z', 'y', 'x'],
    'b': [1, 2, 3, 4, 5, 6],
    'c': ['2024-01-01', '2024-01-02', '2024-01-01', '2024-01-03', '2024-01-02', '2024-01-01']
})

# Parquet
df.to_parquet("tmp/test.parquet", index=False)


