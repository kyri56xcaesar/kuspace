package uspace

import (
	"slices"
	"strings"
	"testing"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

func TestDockerRunArgs(t *testing.T) {
	q, err := parseJobQuotas(ut.Job{MemoryRequest: "256Mi", MemoryLimit: "512Mi", CPURequest: "250m", CPULimit: "1500m"})
	if err != nil {
		t.Fatal(err)
	}
	job := ut.Job{JID: 9, Logic: "kyri56xcaesar/kuspace:applications-bash-v2",
		Env: map[string]string{"OUTPUT_URL": "http://m/o?sig", "INPUT_URL": "http://m/i?sig", "LOGIC": "sort {input} > {output}"}}
	args := dockerRunArgs(job, []string{"python3", "bash_app.py"}, q, "devnet")
	joined := strings.Join(args, " ")

	for _, want := range []string{
		"run --rm --name kuspace-job-9",
		"--memory 536870912", // 512Mi
		"--cpus 1.500",
		"--network devnet",
		"-e INPUT_URL=http://m/i?sig -e LOGIC=sort {input} > {output} -e OUTPUT_URL=http://m/o?sig", // sorted, stable
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q:\n%s", want, joined)
		}
	}
	// image, then the command, last
	if i := slices.Index(args, job.Logic); i != len(args)-3 || args[len(args)-1] != "bash_app.py" {
		t.Errorf("image/command not at the end: %v", args[len(args)-4:])
	}
	for _, a := range args {
		if strings.Contains(a, "SECRET") || strings.Contains(a, "ACCESS_KEY") {
			t.Errorf("credential in docker args: %q", a)
		}
	}
	if slices.Contains(dockerRunArgs(job, nil, q, ""), "--network") {
		t.Error("--network passed without a configured network")
	}
}
