
def run(data):
  	print('hello from inside!, i can execute code?')
    import os
    
    print(os.getcwd())
    
    return data.upper()








with open('/input/test_login1.json', 'r') as input:
	with open('/output/sagaghsahsaashsahsahsa', 'w') as output:
		output.write(run(input.read()))

