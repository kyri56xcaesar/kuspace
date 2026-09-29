package model

// Jobs and the applications catalogue.

import (
	"errors"
	"fmt"
	"strings"

	ut "kyri56xcaesar/kuspace/internal/utils"
)

// Job struct defines all the data required and optional for executing a Job.
type Job struct {
	JID int64 `json:"jid,omitempty" form:"jid"`
	UID int64 `json:"uid" form:"uid"`

	Parallelism int `json:"parallelism,omitempty" form:"parallelism"`
	Priority    int `json:"priority,omitempty" form:"priority"`

	MemoryRequest string `json:"memoryRequest,omitempty" form:"memory_request"`
	CPURequest    string `json:"cpuRequest,omitempty" form:"cpuRequest"`
	MemoryLimit   string `json:"memoryLimit,omitempty" form:"memoryLimit"`
	CPULimit      string `json:"cpuLimit,omitempty" form:"cpuLimit"`

	EphemeralStorageRequest string `json:"ephemeralStorageRequest,omitempty" form:"ephemeralStorageRequest"`
	EphemeralStorageLimit   string `json:"ephemeralStorageLimit,omitempty" form:"ephemeralStorageLimit"`

	Description string  `json:"description,omitempty" form:"description"`
	Duration    float64 `json:"duration,omitempty" form:"duration"`

	Input   string `json:"input" form:"input"`
	Output  string `json:"output" form:"output"`
	Timeout int    `json:"timeout,omitempty" form:"timeout" ` // in minutes

	Env map[string]string `json:"env,omitempty"`

	// perhaps unnecessary
	InputFormat  string `json:"inputFormat,omitempty"`
	OutputFormat string `json:"outputFormat,omitempty"`

	Logic        string   `json:"logic" form:"logic"`
	LogicBody    string   `json:"logicBody" form:"logicBody"`
	LogicHeaders string   `json:"logicHeaders,omitempty" form:"logicHeaders"`
	Params       []string `json:"params,omitempty" form:"params"`

	Status      string `json:"status,omitempty" form:"status"`
	Completed   bool   `json:"completed,omitempty" form:"completed"`
	CompletedAt string `json:"completedAt,omitempty" form:"completedAt"`
	CreatedAt   string `json:"createdAt,omitempty" form:"createdAt"`

	// Engine is where the job ran ("kubernetes" or "docker"); set by uspace,
	// never by the submitter.
	Engine string `json:"engine,omitempty" form:"-"`

	// StatusReason says why the job is in its state when that isn't obvious
	// (re-queued or failed after a restart); Attempts counts how often it was
	// claimed to run. Set by uspace's database dispatcher.
	StatusReason string `json:"statusReason,omitempty" form:"-"`
	Attempts     int    `json:"attempts,omitempty" form:"-"`

	// GID is the owner's primary group at submission: the job's output files
	// get it. Set by uspace, never by the submitter.
	GID int64 `json:"gid,omitempty" form:"-"`
}

// ValidateForm method sanitizes and checks if the given Job object is within limits
func (j *Job) ValidateForm(maxCPU, maxMem, maxStorage, maxParal, maxTimeout, maxChars int64) error {
	// Validate
	if j.Description != "" && !ut.IsValidUTF8String(j.Description) {
		return errors.New("description must contain valid characters")
	}

	if j.LogicHeaders != "" && !ut.IsValidUTF8String(j.LogicHeaders) {
		return errors.New("headers must contain valid characters")
	}

	if j.Logic == "" {
		return errors.New("must provide logic")
	}

	if !ut.IsValidUTF8String(j.Logic) {
		return errors.New("logic must contain valid characters")
	}

	if len(j.LogicBody) == 0 {
		return errors.New("must provide logic")
	}

	if int64(len(j.LogicBody)) > maxChars {
		return fmt.Errorf("logic_body exceeds max length of %v characters", maxChars)
	}

	if j.Params != nil && !ut.IsValidUTF8String(strings.Join(j.Params, " ")) {
		return errors.New("headers must contain valid characters")
	}

	if j.Input == "" {
		return errors.New("must provide input")
	}

	if j.Output == "" {
		return errors.New("must provide output")
	}
	// Validate paths
	if !ut.IsValidPath(j.Input) || !ut.IsValidPath(j.Output) {
		return errors.New("input/output paths contain invalid characters")
	}

	if len(j.Description) > 150 {
		return errors.New("description must be less or equal than 150 characters")
	}

	if j.Parallelism > int(maxParal) || j.Parallelism < 0 {
		return fmt.Errorf("parallelism must be between 0 and %d", maxParal)
	}

	if j.Timeout > int(maxTimeout) {
		return fmt.Errorf("max timeout allowed: %v", maxTimeout)
	}

	// CPU
	cpuReq, err := parseCPU(j.CPURequest)
	if err != nil || cpuReq > float64(maxCPU) {
		return fmt.Errorf("cpu_request must be a float less than %v", maxCPU)
	}

	cpuLim, err := parseCPU(j.CPULimit)
	if err != nil || cpuLim > float64(maxCPU) {
		return fmt.Errorf("cpu_limit must be a float less than %v", maxCPU)
	}

	// Memory
	memReq, err := parseMi(j.MemoryRequest)
	if err != nil || int64(memReq) > maxMem {
		return fmt.Errorf("memory_request must be less than %vMi", maxMem)
	}

	memLim, err := parseMi(j.MemoryLimit)
	if err != nil || int64(memLim) > maxMem {
		return fmt.Errorf("memory_limit must be between less than %vMi", maxMem)
	}

	// Ephemeral Storage
	storageReq, err := parseGi(j.EphemeralStorageRequest)
	if err != nil || storageReq > float64(maxStorage) {
		return fmt.Errorf("ephemeral_storage_request must be less than %vGi", maxStorage)
	}

	storageLim, err := parseGi(j.EphemeralStorageLimit)
	if err != nil || storageLim > float64(maxStorage) {
		return fmt.Errorf("ephemeral_storage_limit must be less than %vGi", maxStorage)
	}

	// sanitize
	j.Input = strings.TrimSpace(j.Input)
	j.Output = strings.TrimSpace(j.Output)

	if p := strings.Split(j.Output, "/"); len(p) != 2 || p[1] == "" {
		r, err := ut.GenerateRandomString(16)
		if err != nil {
			return errors.New("failed to generate output name, must provide an output object name")
		}
		j.Output = j.Output + r
	}

	j.MemoryRequest = appendIfMissing(strings.TrimSpace(j.MemoryRequest), "Mi")
	j.MemoryLimit = appendIfMissing(strings.TrimSpace(j.MemoryLimit), "Mi")

	j.EphemeralStorageRequest = appendIfMissing(strings.TrimSpace(j.EphemeralStorageRequest), "Gi")
	j.EphemeralStorageLimit = appendIfMissing(strings.TrimSpace(j.EphemeralStorageLimit), "Gi")

	j.CPURequest = strings.TrimSpace(j.CPURequest)
	j.CPULimit = strings.TrimSpace(j.CPULimit)

	return nil
}

// PtrFields returns a slice of pointers to the Job struct fields,
// in a specific order. This is useful for scanning database rows directly
// into the struct fields or for generic update operations.
func (j *Job) PtrFields() []any {
	return []any{
		&j.JID, &j.UID, &j.Description, &j.Duration, &j.Input,
		&j.InputFormat, &j.Output, &j.OutputFormat, &j.Logic, &j.LogicBody,
		&j.LogicHeaders, &j.Params, &j.Status, &j.Completed, &j.CompletedAt,
		&j.CreatedAt, &j.Parallelism, &j.Priority, &j.MemoryRequest, &j.CPURequest,
		&j.MemoryLimit, &j.CPULimit, &j.EphemeralStorageRequest, &j.EphemeralStorageLimit,
	}
}

// Fields returns a slice containing the values of the Job struct fields,
// in a specific order suitable for database operations or serialization.
// The returned slice includes all fields, including IDs and metadata.
func (j *Job) Fields() []any {
	return []any{
		j.JID, j.UID, j.Description, j.Duration, j.Input,
		j.InputFormat, j.Output, j.OutputFormat, j.Logic, j.LogicBody,
		j.LogicHeaders, j.Params, j.Status, j.Completed, j.CompletedAt,
		j.CreatedAt, j.Parallelism, j.Priority, j.MemoryRequest, j.CPURequest,
		j.MemoryLimit, j.CPULimit, j.EphemeralStorageRequest, j.EphemeralStorageLimit,
	}
}

// PtrFieldsNoID returns a slice of pointers to the Job struct fields,
// excluding the primary ID (Rid). This is typically used for insert or update
// operations where the ID is auto-generated or not required.
func (j *Job) PtrFieldsNoID() []any {
	return []any{
		&j.UID, &j.Description, &j.Duration, &j.Input,
		&j.InputFormat, &j.Output, &j.OutputFormat, &j.Logic, &j.LogicBody,
		&j.LogicHeaders, &j.Params, &j.Status, &j.Completed, &j.CompletedAt,
		&j.CreatedAt, &j.Parallelism, &j.Priority, &j.MemoryRequest, &j.CPURequest,
		&j.MemoryLimit, &j.CPULimit, &j.EphemeralStorageRequest, &j.EphemeralStorageLimit,
	}
}

// FieldsNoID returns a slice containing the values of the Job struct fields,
// excluding the primary ID (Rid). This is typically used for insert or update
// operations where the ID is auto-generated or not required.
func (j *Job) FieldsNoID() []any {
	return []any{
		j.UID, j.Description, j.Duration, j.Input,
		j.InputFormat, j.Output, j.OutputFormat, j.Logic, j.LogicBody,
		j.LogicHeaders, j.Params, j.Status, j.Completed, j.CompletedAt,
		j.CreatedAt, j.Parallelism, j.Priority, j.MemoryRequest, j.CPURequest,
		j.MemoryLimit, j.CPULimit, j.EphemeralStorageRequest, j.EphemeralStorageLimit,
	}
}

// APIResponse aims to unite the type of responses of microservices , bricking the "Response Model"
type APIResponse[T any] struct {
	Status  string `json:"status"`  // e.g., "success", "error"
	Message string `json:"message"` // e.g., "Operation successful"
	Data    T      `json:"data"`    // Any data payload
}

// Application struct defines the needed information of the "Applications" that are used and exist in the system
type Application struct {
	ID          int64  `json:"id,omitempty" form:"id"`
	Name        string `json:"name" form:"name"`
	Image       string `json:"image" form:"image"`
	Description string `json:"description,omitempty" form:"description"`
	Version     string `json:"version" form:"version"`
	Author      string `json:"author" form:"author"`
	AuthorID    int64  `json:"authorId,omitempty" form:"authorId"`
	Status      string `json:"status" form:"status"`
	InsertedAt  string `json:"insertedAt,omitempty" form:"insertedAt"`
	CreatedAt   string `json:"createdAt,omitempty" form:"createdAt"`
}

// FieldsNoID returns a slice containing the values of the Application struct fields,
// excluding the primary ID . This is typically used for insert or update
// operations where the ID is auto-generated or not required.
func (a *Application) FieldsNoID() []any {
	return []any{a.Name, a.Image, a.Description, a.Version, a.Author, a.AuthorID, a.Status, a.InsertedAt, a.CreatedAt}
}

// Fields returns a slice containing the values of the Application struct fields,
// in a specific order suitable for database operations or serialization.
// The returned slice includes all fields, including IDs and metadata.
func (a *Application) Fields() []any {
	return []any{a.ID, a.Name, a.Image, a.Description, a.Version, a.Author, a.AuthorID, a.Status, a.InsertedAt, a.CreatedAt}
}

// PtrFieldsNoID returns a slice of pointers to the Application struct fields,
// excluding the primary ID (Rid). This is typically used for insert or update
// operations where the ID is auto-generated or not required.
func (a *Application) PtrFieldsNoID() []any {
	return []any{&a.Name, &a.Image, &a.Description, &a.Version, &a.Author, &a.AuthorID, &a.Status, &a.InsertedAt, &a.CreatedAt}
}

// PtrFields returns a slice of pointers to the Application struct fields,
// in a specific order. This is useful for scanning database rows directly
// into the struct fields or for generic update operations.
func (a *Application) PtrFields() []any {
	return []any{&a.ID, &a.Name, &a.Image, &a.Description, &a.Version, &a.Author, &a.AuthorID, &a.Status, &a.InsertedAt, &a.CreatedAt}
}
