x = 1
print(x)
def run(data):
	print('from run')
	return data.upper()

with open('tests/input/job.in', 'r') as input:
	with open('tests/output/job.out', 'w') as output:
		output.write(run(input.read()))

