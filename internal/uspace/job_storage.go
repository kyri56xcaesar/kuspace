package uspace

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"

	"kyri56xcaesar/kuspace/internal/model"
	"kyri56xcaesar/kuspace/internal/uspace/jobs"
	ut "kyri56xcaesar/kuspace/internal/utils"
)

/*
	uspace's side of running jobs (jobs.Storage): the storage work a job
	needs. A job's container reads its input and writes its output through
	presigned URLs; uspace records the output as the owner's file.
*/

// jobStorage implements jobs.Storage for the service.
type jobStorage struct{ srv *UService }

// DefaultVolume is where an input or output named without a volume lives.
func (s jobStorage) DefaultVolume() string { return s.srv.storage.DefaultVolume(false) }

// presigner is implemented by storage backends that can hand out
// per-object URLs (MinIO).
type presigner interface {
	PresignFor(ctx context.Context, method string, r model.Resource, d time.Duration) (*url.URL, error)
}

// PresignJobIO gives a job URLs to GET its input and PUT its output, valid
// for valid (no storage credentials ever reach the job).
func (s jobStorage) PresignJobIO(ctx context.Context, in, out model.Resource, valid time.Duration) (string, string, error) {
	p, ok := s.srv.storage.(presigner)
	if !ok {
		return "", "", errors.New("the storage backend can't issue per-object URLs; jobs need MinIO")
	}
	get, err := p.PresignFor(ctx, "get", in, valid)
	if err != nil {
		return "", "", fmt.Errorf("presign input: %w", err)
	}
	put, err := p.PresignFor(ctx, "put", out, valid)
	if err != nil {
		return "", "", fmt.Errorf("presign output: %w", err)
	}

	return get.String(), put.String(), nil
}

// RecordOutput stats a completed job's output object and records it as the
// owner's file (saveJobOutput): what it did, for the job's log.
func (s jobStorage) RecordOutput(ctx context.Context, job model.Job) (string, error) {
	srv := s.srv
	vname, name, ok := strings.Cut(job.Output, "/")
	if !ok {
		return "", fmt.Errorf("invalid output location %q (%w)", job.Output, ut.ErrInvalid)
	}
	outputResource := model.Resource{
		Name:  name,
		Path:  "/",
		Type:  "file",
		Perms: model.DefaultFilePerms,
		UID:   job.UID,
		Vname: vname,
		VID:   srv.volumeID(ctx, vname),
		GID:   jobs.OutputGID(job),
	}
	info, err := srv.storage.Stat(ctx, outputResource)
	if err != nil {
		return "", fmt.Errorf("retrieving the output file: %w", err)
	}
	obj, ok := info.(minio.ObjectInfo)
	if !ok {
		return "", errors.New("unexpected object info from the storage")
	}
	outputResource.Size = obj.Size
	now := ut.CurrentTime()
	outputResource.CreatedAt, outputResource.UpdatedAt, outputResource.AccessedAt = now, now, now

	return srv.saveJobOutput(ctx, outputResource)
}

// OutputWrittenSince reports whether the job's output object was written
// at or after since: how a job whose container is gone shows it finished.
func (s jobStorage) OutputWrittenSince(ctx context.Context, job model.Job, since time.Time) bool {
	vname, name, ok := strings.Cut(strings.TrimPrefix(job.Output, "/"), "/")
	if !ok {
		return false
	}
	info, err := s.srv.storage.Stat(ctx, model.Resource{Vname: vname, Name: name})
	if err != nil {
		return false
	}
	obj, ok := info.(minio.ObjectInfo)

	return ok && !obj.LastModified.Before(since.Truncate(time.Second))
}

// saveJobOutput records a job's output object: a new one is created (owned
// by the job's owner); an existing one (a job may overwrite an output its
// owner can write, checked at submission) gets its new size and time. Usage
// follows the records, so nothing else is charged or refunded.
func (srv *UService) saveJobOutput(ctx context.Context, output model.Resource) (string, error) {
	quota := min(srv.config.LocalVolumesDefaultCapacity, maxDefaultVolumeCapacity)
	_, found, err := srv.lookupResource(ctx, output.Name, output.Vname)
	if err != nil {
		return "", err
	}
	if !found {
		// recorded even past the quota: the job already wrote it
		if err := srv.fsl.InsertResource(ctx, output, quota, false); err != nil {
			return "", err
		}

		return "created", nil
	}

	if err := srv.fsl.SetObjectSize(ctx, output.Name, output.Vname, output.Size); err != nil {
		return "", err
	}

	return "updated (overwrote the existing file)", nil
}
