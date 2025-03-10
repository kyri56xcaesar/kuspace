package userspace

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

var (
	python_io_skeleton_code string = `%s

with open('%s', 'r') as input:
	with open('%s', 'w') as output:
		output.write(run(input.read()))

`
	node_io_skeleton_code string = `%s

const fs = require('fs');

fs.readFile('%s', 'utf8', (err, data) => {
	if (err) throw err;
	const result = run(data);
	fs.writeFile('%s', result, 'utf8', (err) => {
		if (err) throw err;
	});
});

`
	java_io_skeleton_code string = `%s

import java.nio.file.Files;
import java.nio.file.Paths;

public class Main {
	public static void main(String[] args) throws Exception {
		String input = new String(Files.readAllBytes(Paths.get("%s")));
		String output = run(input);
		Files.write(Paths.get("%s"), output.getBytes());
	}
}

`
	c_io_skeleton_code string = `%s
int main() {
	FILE *in_fp, *out_fp;
	in_fp = fopen("%s", "r");
	if (in_fp == NULL) {
		perror("Error opening input file");
		return 1;
	}
	out_fp = fopen("%s", "w");
	if (out_fp == NULL) {
		perror("Error opening output file");
		return 1;
	}
	
	%s

	fclose(in_fp);
	fclose(out_fp);
}
	`
	go_io_skeleton_code string = `%s

import (
	"io/ioutil"
)

func main() {
	input, err := ioutil.ReadFile("%s")
	if err != nil {
		panic(err)
	}
	output := run(string(input))
	err = ioutil.WriteFile("%s", []byte(output), 0644)
	if err != nil {
		panic(err)
	}
}

`
)

func performExecution(job Job, verbose bool) (*exec.Cmd, error) {
	// ctx := context.Background()
	command := formatJobCommand(job, true)
	if verbose {
		log.Printf("command: %s", command)
	}
	cmd := exec.Command(command)

	return cmd, nil
}

func formatJobCommand(job Job, file bool) string {
	/*	cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve working directory")
		}
			// ctx,
			"docker",
			"run",
			"-v", "'"+cwd+"/tests/input/job.in:/input/job.in"+"'", //input
			"-v", "'"+cwd+"/tests/output:/output"+"'", //output
			"-v", "'"+cwd+fmt.Sprintf("/tests/job-%d.py:/script.py", job.Jid),
			language+":"+version,
			command,
	*/
	inp := strings.Split(job.Input[0], "/")
	out := strings.Split(job.Output, "/")
	parts := strings.Split(job.Logic, ":")
	language := parts[0]
	var version string
	version = "latest"
	if len(parts) == 2 {
		version = parts[1]
	}

	var command []string = []string{"docker", "run"}
	switch language {
	case "python":
		if file {
			err := os.WriteFile(fmt.Sprintf("tmp/job-%d.py", job.Jid), []byte(fmt.Sprintf(python_io_skeleton_code, job.LogicBody, "/input/"+inp[len(inp)-1], "/output/"+out[len(out)-1])), 0666)
			if err != nil {
				log.Printf("failed to write file: %v", err)
				return ""
			}
			return "python3 ./script.py"
		}
		return "python -c " + fmt.Sprintf(python_io_skeleton_code, job.LogicBody, "/input/"+inp[len(inp)-1], "/output/"+out[len(out)-1])

	case "node", "javascript":
		return fmt.Sprintf(node_io_skeleton_code, job.LogicBody, "/input/"+inp[len(inp)-1], "/output/"+out[len(out)-1])
	case "go":
		return fmt.Sprintf(go_io_skeleton_code, job.LogicBody, "/input/"+inp[len(inp)-1], "/output/"+out[len(out)-1])
	case "java":
		return fmt.Sprintf(java_io_skeleton_code, job.LogicBody, "/input/"+inp[len(inp)-1], "/output/"+out[len(out)-1])
	case "c":
		return fmt.Sprintf(c_io_skeleton_code, job.LogicHeaders, "/input/"+inp[len(inp)-1], "/output/"+out[len(out)-1], job.LogicBody)
	default:
		return ""
	}
}
