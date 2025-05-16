import random
import string

# Parameters
num_words = 10**6  # 1 million words
repeated_words = ["hello", "world", "mapreduce", "data", "science"]  # Common words
repeat_frequency = 0.01  # 1% of words should be common

# Generate random words and mix in common words
words = []
for _ in range(num_words):
    if random.random() < repeat_frequency:
        words.append(random.choice(repeated_words))
    else:
        word_length = random.randint(3, 10)
        words.append(''.join(random.choices(string.ascii_lowercase, k=word_length)))

# Save to file
file_path = "tmp/input/test_words.txt"
with open(file_path, "w") as f:
    f.write("\n".join(words))

file_path
