package uspace

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

var (
	defaultVPath         = "data/volumes"
	tmpPath              = "tmp/"
	pythonIoSkeletonCode = `%s

with open('%s', 'r') as input:
	with open('%s', 'w') as output:
		output.write(run(input.read()))

`
	nodeIoSkeletonCode = `%s

const fs = require('fs');

fs.readFile('%s', 'utf8', (err, data) => {
	if (err) throw err;
	fs.writeFile('%s',  run(data), 'utf8', (err) => {
		if (err) throw err;
	});
});

`
	javaIoSkeletonCode = `import java.nio.file.Files;
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
	cIoSkeletonCode = `#include <stdio.h>
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
	goIoSkeletonCode = `package main
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

// JDockerExecutor struct impelementing the JobExecutor interface
// responsible for executing jobs in the docker engine
type JDockerExecutor struct {
	jm *JobManager
}

// NewJDockerExecutor function as a constructor
func NewJDockerExecutor(jm *JobManager) JDockerExecutor {
	defaultVPath = jm.srv.storage.DefaultVolume(true)

	return JDockerExecutor{
		jm: jm,
	}
}

// ExecuteJob method, the core logic of execution
func (je JDockerExecutor) ExecuteJob(job ut.Job) error {
	// log.Printf("executing job: %+v", job)
	defer func() { <-je.jm.workerPool }() // Release worker slot

	je.jm.mu.Lock()
	job.Status = "running"
	je.jm.mu.Unlock()

	// we should examine input "resources"
	// or if exists in the storage
	var asResource ut.Resource
	// inp can be in format <volume>/<path>
	parts := strings.Split(job.Input, "/")
	if len(parts) > 1 {
		asResource.Vname = parts[0]
		asResource.Name = strings.Join(parts[1:], "/")
	} else {
		asResource.Vname = je.jm.srv.storage.DefaultVolume(false)
		asResource.Name = job.Input
	}

	_, err := je.jm.srv.storage.Stat(asResource)
	if err != nil {
		log.Printf("failed to find input resource: %v", err)

		return err
	}

	// language and version
	cmd, duration, err := prepareExecution(job, true)
	if err != nil {
		log.Printf("failed to prepare or perform job: %v", err)

		return err
	}
	job.Duration = duration.Abs().Seconds()

	// output should be streamed back ...
	wsChan := make(chan []byte, 100)
	// ws_chan_err := make(chan []byte, 100)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("error creating stdout pipe: %v", err)

		return err
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		wsChan <- []byte(line)
	}

	// Start the command
	log.Printf("starting job execution")
	if err := cmd.Start(); err != nil {
		log.Printf("error starting command: %v", err)
		updateJobStatus(&je, job.JID, "failed", 0)

		return err
	}
	log.Printf("streaming to socket")
	go streamToSocketWS(job.JID, wsChan)

	log.Printf("waiting...")
	err = cmd.Wait()
	if err != nil {
		log.Printf("Job %d failed: %s\n", job.JID, err)
		// je.updateJobStatus(job.JID, "failed", 0)
	}

	// success
	log.Printf("Job %d completed successfully\n", job.JID)
	// je.updateJobStatus(job.JID, "completed", duration)

	// // insert the output resource
	// go je.syncOutputResource(job)

	// should cleanup the tmps, etc..
	// lets cleanup the debree
	cleanup(job.JID, true, job.Logic)

	return err
}

// CancelJob method, the logic that halts job execution
func (je JDockerExecutor) CancelJob(_ ut.Job) error {
	return nil
}

func prepareExecution(job ut.Job, verbose bool) (*exec.Cmd, time.Duration, error) {
	// ctx := context.Background()
	command, err := formatJobCommandD(job, true)
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

func formatJobCommandD(job ut.Job, fileSave bool) ([]string, error) {
	/*
		// ctx,
		"docker",
		"run",
		"-v", "'"+cwd+"/tests/input/job.in:/input/job.in"+"'", //input
		"-v", "'"+cwd+"/tests/output:/output"+"'", //output
		"-v", "'"+cwd+fmt.Sprintf("/tests/job-%d.py:/script.py", job.JID),
		language+":"+version,
		command,
	*/
	cwd, err := os.Getwd()
	if err != nil {
		return nil, errors.New("failed to retrieve working directory")
	}

	// how should we handle multiple input files? 1] lets combine (append) them to a single file for now...

	inp := fmt.Sprintf("input-%d", job.JID)
	out := strings.Split(job.Output, "/")
	parts := strings.Split(job.Logic, ":")
	language := parts[0]
	var version string
	version = "latest"
	if len(parts) == 2 {
		version = parts[1]
	}

	command := []string{"run"}
	switch language {
	case "python":
		if fileSave {
			err := os.WriteFile(fmt.Sprintf("tmp/job-%d.py", job.JID), []byte(fmt.Sprintf(pythonIoSkeletonCode, job.LogicBody, "/input/"+inp, "/output/"+out[len(out)-1])), 0o644)
			if err != nil {
				log.Printf("failed to write file: %v", err)

				return nil, fmt.Errorf("failed to write tmp file script: %w", err)
			}
			command = append(command, []string{
				"-v", cwd + "/" + tmpPath + inp + ":/input/" + inp, // input
				"-v", cwd + "/" + defaultVPath + "output:/output", // output,
				"-v", cwd + fmt.Sprintf("/tmp/job-%d.py", job.JID) + ":/script.py", // script to run
				language + ":" + version, // image
				"python", "./script.py",
				"ls", "-lrth",
			}...)

			return command, nil
		}
		// return "python -c " +
		// fmt.Sprintf(pythonIOSkeletonCode, job.LogicBody, "/input/"+inp[len(inp)-1], "/output/"+out[len(out)-1])

	case "node", "javascript":
		language = "node"
		if fileSave {
			err := os.WriteFile(fmt.Sprintf("tmp/job-%d.js", job.JID), []byte(fmt.Sprintf(nodeIoSkeletonCode, job.LogicBody, "/input/"+inp, "/output/"+out[len(out)-1])), 0o644)
			if err != nil {
				log.Printf("failed to write file: %v", err)

				return nil, fmt.Errorf("failed to write tmp file script: %w", err)
			}
			command = append(command, []string{
				"-v", cwd + "/" + tmpPath + inp + ":/input/" + inp, // input
				"-v", cwd + "/" + defaultVPath + "/output:/output", // output,
				"-v", cwd + fmt.Sprintf("/tmp/job-%d.js", job.JID) + ":/script.js", // script to run
				language + ":" + version, // image
				"node", "./script.js",
			}...)

			return command, nil
		}
	case "go", "golang":
		language = "golang"
		if fileSave {
			err := os.WriteFile(fmt.Sprintf("tmp/job-%d.go", job.JID), []byte(fmt.Sprintf(goIoSkeletonCode, job.LogicBody, "/input/"+inp, "/output/"+out[len(out)-1])), 0o644)
			if err != nil {
				log.Printf("failed to write file: %v", err)

				return nil, fmt.Errorf("failed to write tmp file script: %w", err)
			}
			command = append(command, []string{
				"-v", cwd + "/" + tmpPath + inp + ":/input/" + inp, // input
				"-v", cwd + "/" + defaultVPath + "/output:/output", // output,
				"-v", cwd + fmt.Sprintf("/tmp/job-%d.go", job.JID) + ":/script.go", // script to run
				language + ":" + version, // image
				"go", "run", "/script.go",
			}...)

			return command, nil
		}
	case "openjdk", "java": // java
		language = "openjdk"
		if fileSave {
			err := os.WriteFile(fmt.Sprintf("tmp/job-%d.java", job.JID), []byte(fmt.Sprintf(javaIoSkeletonCode, job.LogicHeaders, job.LogicBody, "/input/"+inp, "/output/"+out[len(out)-1])), 0o644)
			if err != nil {
				log.Printf("failed to write file: %v", err)

				return nil, fmt.Errorf("failed to write tmp file script: %w", err)
			}
			command = append(command, []string{
				"-v", cwd + "/" + tmpPath + inp + ":/input/" + inp, // input
				"-v", cwd + "/" + defaultVPath + "/output:/output", // output,
				"-v", cwd + fmt.Sprintf("/tmp/job-%d.java", job.JID) + ":/Main.java", // script to run
				language + ":" + version,      // image
				"sh", "-c", "java /Main.java", // compile and run
			}...)
			time.Sleep(1 * time.Second)

			return command, nil
		}
	case "c", "gcc":
		if fileSave {
			err := os.WriteFile(fmt.Sprintf("tmp/job-%d.c", job.JID), []byte(fmt.Sprintf(cIoSkeletonCode, job.LogicHeaders, job.LogicBody, "/input/"+inp, "/output/"+out[len(out)-1])), 0o644)
			if err != nil {
				log.Printf("failed to write file: %v", err)

				return nil, fmt.Errorf("failed to write tmp file script: %w", err)
			}
			command = append(command, []string{
				"-v", cwd + "/" + tmpPath + inp + ":/input/" + inp, // input
				"-v", cwd + "/" + defaultVPath + "/output:/output", // output,
				"-v", cwd + fmt.Sprintf("/tmp/job-%d.c", job.JID) + ":/program.c", // script to run
				language + ":" + version,                             // image
				"sh", "-c", "gcc /program.c -o program && ./program", // compile and run
			}...)
			time.Sleep(1 * time.Second)

			return command, nil
		}
	default:
		log.Printf("language: %s", language)

		return nil, errors.New("unrecognised/unsupported language")
	}

	return nil, errors.New("bad state")
}

func updateJobStatus(je *JDockerExecutor, jid int64, status string, duration time.Duration) {
	log.Printf("updating %v job status: %v", jid, status)
	err := je.jm.srv.markJobStatus(jid, status, duration)
	if err != nil {
		log.Printf("failed to update job %d status (%s): %v", jid, status, err)
	}
}

func syncOutputResource(je *JDockerExecutor, job ut.Job) {
	fInfo, err := os.Stat(defaultVPath + "/output/" + job.Output)
	if err != nil {
		log.Printf("failed to find/stat the output file: %v", err)

		return
	}

	currentTime := ut.CurrentTime()
	resource := ut.Resource{
		Name:       "/output/" + job.Output,
		Type:       "file",
		CreatedAt:  currentTime,
		UpdatedAt:  currentTime,
		AccessedAt: currentTime,
		Perms:      "rw-r--r--",
		RID:        0,
		UID:        job.UID,
		VID:        0,
		GID:        job.UID,
		Size:       fInfo.Size(),
		Links:      0,
	}

	cancelFn, err := je.jm.srv.storage.Insert([]any{resource})
	defer cancelFn()
	if err != nil {
		log.Printf("failed to insert the resource")
	}
}

func cleanup(jid int64, verbose bool, language string) {
	// remove the tmp files
	err := os.Remove(fmt.Sprintf("tmp/input-%d", jid))
	if err != nil && verbose {
		log.Printf("failed to remove tmp file: %v", err)
	}

	if strings.Contains(language, "python") {
		err = os.Remove(fmt.Sprintf("tmp/job-%d.py", jid))
		if err != nil && verbose {
			log.Printf("failed to remove tmp file: %v", err)
		}
	} else if strings.Contains(language, "node") || strings.Contains(language, "javascript") {
		err = os.Remove(fmt.Sprintf("tmp/job-%d.js", jid))
		if err != nil && verbose {
			log.Printf("failed to remove tmp file: %v", err)
		}
	} else if strings.Contains(language, "go") || strings.Contains(language, "golang") {
		err = os.Remove(fmt.Sprintf("tmp/job-%d.go", jid))
		if err != nil && verbose {
			log.Printf("failed to remove tmp file: %v", err)
		}
	} else if strings.Contains(language, "openjdk") || strings.Contains(language, "java") {
		err = os.Remove(fmt.Sprintf("tmp/job-%d.java", jid))
		if err != nil && verbose {
			log.Printf("failed to remove tmp file: %v", err)
		}
	} else if strings.Contains(language, "c") || strings.Contains(language, "gcc") {
		err = os.Remove(fmt.Sprintf("tmp/job-%d.c", jid))
		if err != nil && verbose {
			log.Printf("failed to remove tmp file: %v", err)
		}
	} else if verbose {
		log.Printf("no such language support")
	}
}
