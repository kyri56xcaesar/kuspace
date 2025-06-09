from collections import defaultdict
import json

def shuffler(mapped_entries):
    shuffled = defaultdict(list)
    for key, value in mapped_entries:
        shuffled[key].append(value)
    
    return list(shuffled.items())

def reducer(entries):
    accumulator = {}
    for key, values in entries:  # `values` is already a list of 1s
        accumulator[key] = sum(values)  # Summing all occurrences
    
    return accumulator

def mapper(arr):
	
    return [(word, 1) for word in arr]


def run(data):
    
    return json.dumps(reducer(shuffler(mapper(data.split()))))









