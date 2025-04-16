package userspace

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	ut "kyri56xcaesar/myThesis/internal/utils"
)

var (
	default_v_path          string = "data/volumes"
	python_io_skeleton_code string = `%s

with open('%s', 'r') as input:
	with open('%s', 'w') as output:
		output.write(run(input.read()))

`
	node_io_skeleton_code string = `%s

const fs = require('fs');

fs.readFile('%s', 'utf8', (err, data) => {
	if (err) throw err;
	fs.writeFile('%s',  run(data), 'utf8', (err) => {
		if (err) throw err;
	});
});

`
	java_io_skeleton_code string = `import java.nio.file.Files;
import java.nio.file.Paths;
%s
public class Main {
%s
	public static void main(String[] args) throws Exception {
		String input = new String(Files.readAllBytes(Paths.get("%s")));
		String output = run(input);
		Files.write(Paths.get("%s"), output.getBytes());
	}
}

`
	c_io_skeleton_code string = `#include <stdio.h>
#define BUFFER_SIZE 1024
%s
%s
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
	char buffer[BUFFER_SIZE];
 	while (fgets(buffer, BUFFER_SIZE, in_fp) != NULL) {
    	run(buffer);         // Convert to uppercase
    	fputs(buffer, out_fp);  // Write to output file
	}
	fclose(in_fp);
	fclose(out_fp);
	return 0;
}`
	go_io_skeleton_code string = `package main
	import "os"
%s

func main() {
	input, err := os.ReadFile("%s")
	if err != nil {
		panic(err)
	}
	err = os.WriteFile("%s", []byte(run(string(input))), 0644)
	if err != nil {
		panic(err)
	}
}

`
)

type JDockerExecutor struct {
	jm *JobManager
}

func NewJDockerExecutor(jm *JobManager) JDockerExecutor {
	return JDockerExecutor{
		jm: jm,
	}
}

func (je JDockerExecutor) ExecuteJob(job ut.Job) error {
	// log.Printf("executing job: %+v", job)
	defer func() { <-je.jm.workerPool }() // Release worker slot

	je.jm.mu.Lock()
	job.Status = "running"
	je.jm.mu.Unlock()

	// we should examine input "resources"

	// language and version
	default_v_path = je.jm.srv.config.VOLUMES_PATH
	cmd, duration, err := prepareExecution(job, true)
	if err != nil {
		log.Printf("failed to prepare or perform job: %v", err)
		return err
	}
	job.Duration = duration.Abs().Seconds()

	// output should be streamed back ...
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("error creating stdout pipe: %v", err)
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("error creating stderr pipe: %v", err)
		return err
	}

	// Start the command
	log.Printf("starting job execution")
	if err := cmd.Start(); err != nil {
		log.Printf("error starting command: %v", err)
		je.updateJobStatus(job.Jid, "failed", 0)
		return err
	}
	log.Printf("streaming to socket")
	go streamToSocketWS(job.Jid, stdout)
	go streamToSocketWS(job.Jid, stderr)

	log.Printf("waiting...")
	err = cmd.Wait()
	if err != nil {
		log.Printf("Job %d failed: %s\n", job.Jid, err)
		je.updateJobStatus(job.Jid, "failed", 0)
		return err
	}

	log.Printf("Job %d completed successfully\n", job.Jid)
	je.updateJobStatus(job.Jid, "completed", duration)

	// insert the output resource
	go je.syncOutputResource(job)

	// should cleanup the tmps, etc..
	return nil
}

func (je JDockerExecutor) CancelJob(job ut.Job) error {
	return nil
}

func prepareExecution(job ut.Job, verbose bool) (*exec.Cmd, time.Duration, error) {
	// ctx := context.Background()
	command, err := formatJobCommand(job, true)
	if err != nil {
		log.Printf("failed to format command: %v", err)
		return nil, 0, err
	}
	if verbose {
		log.Printf("command: %s", command)
	}
	start := time.Now()
	cmd := exec.Command("docker", command...)

	return cmd, time.Since(start), nil
}

func formatJobCommand(job ut.Job, fileSave bool) ([]string, error) {
	/*
		// ctx,
		"docker",
		"run",
		"-v", "'"+cwd+"/tests/input/job.in:/input/job.in"+"'", //input
		"-v", "'"+cwd+"/tests/output:/output"+"'", //output
		"-v", "'"+cwd+fmt.Sprintf("/tests/job-%d.py:/script.py", job.Jid),
		language+":"+version,
		command,
	*/
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve working directory")
	}
	// how should we handle multiple input files? 1] lets combine (append) them to a single file for now...
	err = ut.MergeFiles(default_v_path+"/input", default_v_path+"/", job.Input)
	if err != nil {
		log.Printf("failed to merge input files: %v", err)
		return nil, err
	}

	inp := "input"
	out := strings.Split(job.Output, "/")
	parts := strings.Split(job.Logic, ":")
	language := parts[0]
	var version string
	version = "latest"
	if len(parts) == 2 {
		version = parts[1]
	}

	var command []string = []string{"run"}
	switch language {
	case "python":
		if fileSave {
			err := os.WriteFile(fmt.Sprintf("tmp/job-%d.py", job.Jid), []byte(fmt.Sprintf(python_io_skeleton_code, job.LogicBody, "/input/"+inp, "/output/"+out[len(out)-1])), 0o644)
			if err != nil {
				log.Printf("failed to write file: %v", err)
				return nil, fmt.Errorf("failed to write tmp file script: %v", err)
			}
			command = append(command, []string{
				"-v", cwd + "/" + default_v_path + "/" + inp + ":/input/" + inp, // input
				"-v", cwd + "/" + default_v_path + "/output:/output", // output,
				"-v", cwd + fmt.Sprintf("/tmp/job-%d.py", job.Jid) + ":/script.py", // script to run
				language + ":" + version, // image
				"python", "./script.py",
			}...)

			return command, nil
		}
		// return "python -c " + fmt.Sprintf(python_io_skeleton_code, job.LogicBody, "/input/"+inp[len(inp)-1], "/output/"+out[len(out)-1])

	case "node", "javascript":
		language = "node"
		if fileSave {
			err := os.WriteFile(fmt.Sprintf("tmp/job-%d.js", job.Jid), []byte(fmt.Sprintf(node_io_skeleton_code, job.LogicBody, "/input/"+inp, "/output/"+out[len(out)-1])), 0o644)
			if err != nil {
				log.Printf("failed to write file: %v", err)
				return nil, fmt.Errorf("failed to write tmp file script: %v", err)
			}
			command = append(command, []string{
				"-v", cwd + "/" + default_v_path + "/" + inp + ":/input/" + inp, // input
				"-v", cwd + "/" + default_v_path + "/output:/output", // output,
				"-v", cwd + fmt.Sprintf("/tmp/job-%d.js", job.Jid) + ":/script.js", // script to run
				language + ":" + version, // image
				"node", "./script.js",
			}...)

			return command, nil
		}
	case "go", "golang":
		language = "golang"
		if fileSave {
			err := os.WriteFile(fmt.Sprintf("tmp/job-%d.go", job.Jid), []byte(fmt.Sprintf(go_io_skeleton_code, job.LogicBody, "/input/"+inp, "/output/"+out[len(out)-1])), 0o644)
			if err != nil {
				log.Printf("failed to write file: %v", err)
				return nil, fmt.Errorf("failed to write tmp file script: %v", err)
			}
			command = append(command, []string{
				"-v", cwd + "/" + default_v_path + "/" + inp + ":/input/" + inp, // input
				"-v", cwd + "/" + default_v_path + "/output:/output", // output,
				"-v", cwd + fmt.Sprintf("/tmp/job-%d.go", job.Jid) + ":/script.go", // script to run
				language + ":" + version, // image
				"go", "run", "/script.go",
			}...)

			return command, nil
		}
	case "openjdk", "java": // java
		language = "openjdk"
		if fileSave {
			err := os.WriteFile(fmt.Sprintf("tmp/job-%d.java", job.Jid), []byte(fmt.Sprintf(java_io_skeleton_code, job.LogicHeaders, job.LogicBody, "/input/"+inp, "/output/"+out[len(out)-1])), 0o644)
			if err != nil {
				log.Printf("failed to write file: %v", err)
				return nil, fmt.Errorf("failed to write tmp file script: %v", err)
			}
			command = append(command, []string{
				"-v", cwd + "/" + default_v_path + "/" + inp + ":/input/" + inp, // input
				"-v", cwd + "/" + default_v_path + "/output:/output", // output,
				"-v", cwd + fmt.Sprintf("/tmp/job-%d.java", job.Jid) + ":/Main.java", // script to run
				language + ":" + version,      // image
				"sh", "-c", "java /Main.java", // compile and run
			}...)
			time.Sleep(1 * time.Second)

			return command, nil
		}
	case "c", "gcc":
		if fileSave {
			err := os.WriteFile(fmt.Sprintf("tmp/job-%d.c", job.Jid), []byte(fmt.Sprintf(c_io_skeleton_code, job.LogicHeaders, job.LogicBody, "/input/"+inp, "/output/"+out[len(out)-1])), 0o644)
			if err != nil {
				log.Printf("failed to write file: %v", err)
				return nil, fmt.Errorf("failed to write tmp file script: %v", err)
			}
			command = append(command, []string{
				"-v", cwd + "/" + default_v_path + "/" + inp + ":/input/" + inp, // input
				"-v", cwd + "/" + default_v_path + "/output:/output", // output,
				"-v", cwd + fmt.Sprintf("/tmp/job-%d.c", job.Jid) + ":/program.c", // script to run
				language + ":" + version,                             // image
				"sh", "-c", "gcc /program.c -o program && ./program", // compile and run
			}...)
			time.Sleep(1 * time.Second)

			return command, nil
		}
	default:
		log.Printf("language: %s", language)
		return nil, fmt.Errorf("unrecognised/unsupported language")
	}

	return nil, fmt.Errorf("bad state")
}

func (je *JDockerExecutor) updateJobStatus(jid int, status string, duration time.Duration) {
	log.Printf("updating %v job status: %v", jid, status)
	err := je.jm.srv.MarkJobStatus(jid, status, duration)
	if err != nil {
		log.Printf("failed to update job %d status (%s): %v", jid, status, err)
	}

}

func (je *JDockerExecutor) syncOutputResource(job ut.Job) {
	fInfo, err := os.Stat(je.jm.srv.config.VOLUMES_PATH + "/output/" + job.Output)
	if err != nil {
		log.Printf("failed to find/stat the output file: %v", err)
		return
	}

	current_time := time.Now().UTC().Format("2006-01-02 15:04:05-07:00")
	resource := ut.Resource{
		Name:        "/output/" + job.Output,
		Type:        "file",
		Created_at:  current_time,
		Updated_at:  current_time,
		Accessed_at: current_time,
		Perms:       "rw-r--r--",
		Rid:         0,
		Uid:         job.Uid,
		Vid:         0,
		Gid:         job.Uid,
		Size:        fInfo.Size(),
		Links:       0,
	}

	err = je.jm.srv.storage.Insert([]any{resource})
	if err != nil {
		log.Printf("failed to insert the resource")
	}
}
