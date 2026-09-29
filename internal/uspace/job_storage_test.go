package uspace

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"kyri56xcaesar/kuspace/internal/model"
)

// presigningStorage is test storage that can hand out per-object URLs.
type presigningStorage struct {
	*memStorage
	calls []string
}

func (p *presigningStorage) PresignFor(_ context.Context, method string, r model.Resource, d time.Duration) (*url.URL, error) {
	p.calls = append(p.calls, method+" "+r.Vname+"/"+r.Name+" "+d.String())

	return url.Parse("http://minio:9000/" + r.Vname + "/" + r.Name + "?X-Amz-Signature=sig")
}

// A job gets a GET for its input and a PUT for its output, nothing else.
func TestPresignJobIO(t *testing.T) {
	p := &presigningStorage{memStorage: &memStorage{objects: map[string][]byte{}}}
	s := jobStorage{srv: &UService{storage: p}}
	in := model.Resource{Vname: "vol", Name: "in.csv"}
	out := model.Resource{Vname: "vol", Name: "out.csv"}
	get, put, err := s.PresignJobIO(t.Context(), in, out, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(get, "/vol/in.csv") || !strings.Contains(put, "/vol/out.csv") {
		t.Errorf("urls = %s | %s", get, put)
	}
	if len(p.calls) != 2 || p.calls[0] != "get vol/in.csv 1h0m0s" || p.calls[1] != "put vol/out.csv 1h0m0s" {
		t.Errorf("presign calls = %v (want a GET for the input and a PUT for the output only)", p.calls)
	}
	// a backend that can't presign is an error, never a fallback to credentials
	plain := jobStorage{srv: &UService{storage: &memStorage{objects: map[string][]byte{}}}}
	if _, _, err := plain.PresignJobIO(t.Context(), in, out, time.Hour); err == nil {
		t.Error("non-presigning storage accepted")
	}
}

func TestSubmissionChecksJobQuotas(t *testing.T) {
	if err := validateJobQuotas(model.Job{MemoryRequest: "512Mi", MemoryLimit: "1Gi", CPURequest: "500m", CPULimit: "abc"}); err == nil {
		t.Error("submission check accepted an invalid cpu limit")
	}
	if err := validateJobQuotas(model.Job{}); err != nil {
		t.Errorf("empty quotas (defaults apply later) rejected: %v", err)
	}
}
