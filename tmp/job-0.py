
def run(data):
    return data.upper()








with open('/input/test_login1.json', 'r') as input:
	with open('/output/output.out', 'w') as output:
		output.write(run(input.read()))

