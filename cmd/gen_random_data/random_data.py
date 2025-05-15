import random
import string

def generate_random_data(num_lines, line_length):
    data = []
    for _ in range(num_lines):
        line = ''.join(random.choices(string.ascii_letters + string.digits, k=line_length))
        data.append(line)
    return data

def write_to_file(filename, data):
    with open(filename, 'w') as file:
        for line in data:
            file.write(line + '\n')

def generate_random_data_to_file(num_lines, line_length, filename):
    with open(filename, 'w') as file:
        for _ in range(num_lines):
            line = ''.join(random.choices(string.ascii_letters + string.digits, k=line_length))
            file.write(line + '\n')

if __name__ == "__main__":
    num_lines = 4000000  # Number of lines of random data
    line_length = 1000  # Length of each line
    filename = '/home/kyri/Documents/kuspace/tmp/big_data_4G.text'
    
    generate_random_data_to_file(num_lines, line_length, filename)