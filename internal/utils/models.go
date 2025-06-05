// Package utils provides core data structures and utility methods for the Userspace API,
// modeling users, groups, resources, volumes, jobs, and access control mechanisms.
//
// This package defines the foundational models used throughout the system, including:
//   - Resource: Represents a file-system-like entity with permissions, ownership, and metadata.
//   - Volume: Models a physical or logical storage volume, including capacity and usage.
//   - User, Group: Represent system users and groups, including membership and credentials.
//   - Job: Encapsulates computational jobs with resource requirements and execution metadata.
//   - AccessClaim: Carries user and group context for access control decisions.
//   - Permissions, PermTriplet: Parse and represent UNIX-like permission schemes.
//
// The package also provides utility methods for validation, permission checks, and
// conversion between struct fields and generic representations, facilitating database
// operations and API interactions.
//
// All models are designed to be serializable to JSON for API communication, and include
// helper methods for common operations such as permission checking, ownership validation,
// and string representations.
//
// This package is intended to be imported by higher-level application logic and API handlers
// to enforce access control, manage resources, and maintain consistent user and group metadata.
package utils

/* every struct used in this system */

/*
* Core structures for the Userspace API and their methods.
*
* perhaps some utility functions as well.
*
* */
import (
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"unicode"
)

// Resource struct
/*
* trying to mimic inodes from a regular fs...
*
* NOTE: Question: How does "executable" permission as a concept hold in this "smwhat cloud" context?
* Read/Write should be there.
*
*
* each resource is owned by a user, a group, a volume, a parent... Perhaps the term shared is more appropriate..
* even though there is singularity in ownership since only one group can own a file... (feels wierd)
*
* NOTE: Question: What relation should the "parent" resource have with its "child"?
* I would guess solely as in reference hierarchy. Perhaps give it some common access permissions as well...
*
*
*
* */
type Resource struct {
	Name string `json:"name"`
	Path string `json:"path,omitempty"`
	Type string `json:"type,omitempty"`

	CreatedAt  string `json:"createdAt,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
	AccessedAt string `json:"accessed_at,omitempty"`

	Perms string `json:"perms,omitempty"`

	Rid int `json:"rid,omitempty"`
	UID int `json:"uid,omitempty"` // as in user id (owner)
	Gid int `json:"gid,omitempty"` // as in group id

	Size  int64 `json:"size,omitempty"`
	Links int   `json:"links,omitempty"`

	Vid   int    `json:"vid,omitempty"`
	Vname string `json:"vname,omitempty"`

	Reader io.Reader `json:"reader,omitempty"`
}

/* representative utility methods of the above structures */
/* fields and ptrs fields for "resource" struct helper methods*/

// Fields returns a slice containing the values of the Resource struct fields,
// in a specific order suitable for database operations or serialization.
// The returned slice includes all fields, including IDs and metadata.
func (r *Resource) Fields() []any {
	return []any{r.Rid, r.UID, r.Gid, r.Vid, r.Vname, r.Size, r.Links, r.Perms, r.Name, r.Path, r.Type, r.CreatedAt, r.UpdatedAt, r.AccessedAt}
}

// PtrFields returns a slice of pointers to the Resource struct fields,
// in a specific order. This is useful for scanning database rows directly
// into the struct fields or for generic update operations.
func (r *Resource) PtrFields() []any {
	return []any{&r.Rid, &r.UID, &r.Gid, &r.Vid, &r.Vname, &r.Size, &r.Links, &r.Perms, &r.Name, &r.Path, &r.Type, &r.CreatedAt, &r.UpdatedAt, &r.AccessedAt}
}

// FieldsNoID returns a slice containing the values of the Resource struct fields,
// excluding the primary ID (Rid). This is typically used for insert or update
// operations where the ID is auto-generated or not required.
func (r *Resource) FieldsNoID() []any {
	return []any{r.UID, r.Gid, r.Vid, r.Vname, r.Size, r.Links, r.Perms, r.Name, r.Path, r.Type, r.CreatedAt, r.UpdatedAt, r.AccessedAt}
}

// PtrFieldsNoID returns a slice of pointers to the Resource struct fields,
// excluding the primary ID (Rid). This is typically used for insert or update
// operations where the ID is auto-generated or not required.
func (r *Resource) PtrFieldsNoID() []any {
	return []any{&r.UID, &r.Gid, &r.Vid, &r.Vname, &r.Size, &r.Links, &r.Perms, &r.Name, &r.Path, &r.Type, &r.CreatedAt, &r.UpdatedAt, &r.AccessedAt}
}

/* this method belongs to the Resource objects
*  and checks if a user claim it allowed to acccess it
*
*  -> authorization.
* */

// HasAccess method checks whether the given AccessClaim applies Read authorization upon the Resource Object
func (r *Resource) HasAccess(userInfo AccessClaim) bool {
	/* parse permissions
	*  rwx     rwx     rwx       644, (unix inode metadata)
	*   |       |       |
	* owner   group   others
	* */

	var perm Permissions
	err := perm.FillFromStr(r.Perms)
	if err != nil {
		log.Printf("failed to retrieve permissions normally: %v", err)
		return false
	}

	/* check ownership
	* if true, can exit prematurely
	* */
	if uid, err := strconv.Atoi(userInfo.UID); err != nil {
		log.Printf("error atoing user id")
		return false
	} else if uid == r.UID {
		return perm.Owner.Read
	}

	/* check groups */
	gids := strings.Split(userInfo.Gids, ",")

	for _, gid := range gids {
		if igid, err := strconv.Atoi(gid); err != nil {
			log.Printf("error atoing group id")
			return false
		} else if igid == r.Gid {
			return perm.Group.Read
		}
	}
	/* check others */
	return perm.Other.Read
}

// HasWriteAccess method checks whether the given AccessClaim applies Write authorization upon the Resource object
/* similar as above just for write access*/
func (r *Resource) HasWriteAccess(userInfo AccessClaim) bool {
	/* parse permissions
	*  rwx     rwx     rwx       644, (unix inode metadata)
	*   |       |       |
	* owner   group   others
	* */
	var perm Permissions
	err := perm.FillFromStr(r.Perms)
	if err != nil {
		log.Printf("failed to retrieve permissions normally: %v", err)
		return false
	}

	/* check ownership
	* if true, can exit prematurely
	* */
	if uid, err := strconv.Atoi(userInfo.UID); err != nil {
		log.Printf("error atoing user id")
		return false
	} else if uid == r.UID {
		return perm.Owner.Write
	}

	/* check groups */
	gids := strings.Split(userInfo.Gids, ",")

	for _, gid := range gids {
		if igid, err := strconv.Atoi(gid); err != nil {
			log.Printf("error atoing group id")
			return false
		} else if igid == r.Gid {
			return perm.Group.Write
		}
	}
	/* check others */
	return perm.Other.Write
}

// HasExecutionAccess method checks the given AccessClaim applies Execution authorization upon the Resource object
/* execution access is somewhat trivial at this point, perhaps it can be used in the future*/
func (r *Resource) HasExecutionAccess(_ AccessClaim) bool {
	return false
}

// IsOwner method will check the given AccessClaim applies Ownership authorization upon the Resource object
// this shall check if the resource owner is of the claim OR if the resource group ownership is included in the claim groups
func (r *Resource) IsOwner(ac AccessClaim) bool {
	intUID, err := strconv.Atoi(ac.UID)
	if err != nil {
		log.Printf("[ownership-controller] failed to atoi access_claim")
		return false
	} else if r.UID == intUID {
		log.Printf("[ownership-controller] user id %d matches item id %d", intUID, r.UID)
		return true
	}

	intGids, err := SplitToInt(strings.TrimSpace(ac.Gids), ",")
	if err != nil {
		log.Printf("[ownership-controller] failed to atoi group ids")
		return false
	}
	for _, gid := range intGids {
		if gid == r.Gid {
			log.Printf("[ownership-controller] user group id %d allows item group %d", gid, r.Gid)
			return true
		}
	}
	// check for group ownership
	return false
}

// Permissions struct describes a unix style file inode permission set
type Permissions struct {
	Representation string      `json:"perms"`
	Owner          PermTriplet `json:"owner"`
	Group          PermTriplet `json:"group"`
	Other          PermTriplet `json:"other"`
}

// FillFromStr method will build the object accordingly from its string representation
/* this method goal is from the given argument "representation" which
* looks like "r-x---r--" (or w.e) to transform into the boolean
* */
func (p *Permissions) FillFromStr(representation string) error {
	if len(representation) != 9 {
		log.Printf("fill fail, invalid representation input")
		return fmt.Errorf("invalid representation")
	}

	chunks := [3]string{
		representation[:3],
		representation[3:6],
		representation[6:],
	}

	parseTriplet := func(triplet string) (PermTriplet, error) {
		if len(triplet) != 3 {
			log.Printf("invalid tripled input length")
			return PermTriplet{}, fmt.Errorf("invalid triplet")
		}
		return PermTriplet{
			Read:    triplet[0] == 'r',
			Write:   triplet[1] == 'w',
			Execute: triplet[2] == 'x',
		}, nil
	}
	var err error
	p.Owner, err = parseTriplet(chunks[0])
	if err != nil {
		log.Printf("failed to parse owner triplet: %v", err)
		return err
	}
	p.Group, err = parseTriplet(chunks[1])
	if err != nil {
		log.Printf("failed to parse group triplet: %v", err)
		return err
	}
	p.Other, err = parseTriplet(chunks[2])
	if err != nil {
		log.Printf("failed to parse other triplet: %v", err)
		return err
	}

	return nil
}

// ToString method formats and returns the permission object to a string
func (p Permissions) ToString() string {
	return fmt.Sprintf("%s%s%s", p.Owner.ToString(), p.Group.ToString(), p.Other.ToString())
}

// PermTriplet a unit of permission description, unix style
type PermTriplet struct {
	Title   string `json:"title"`
	Read    bool   `json:"read"`
	Write   bool   `json:"write"`
	Execute bool   `json:"execute"`
}

// ToString method formats and returns the permission object to a string
func (pt PermTriplet) ToString() string {
	r, w, x := "-", "-", "-"

	if pt.Read {
		r = "r"
	}

	if pt.Write {
		w = "w"
	}

	if pt.Execute {
		x = "x"
	}

	return fmt.Sprintf("%v%v%v", r, w, x)
}

/* This will represent the physical volumes provides by Kubernetes and supervised by the controller */

// Volume struct describes the volume information needed
type Volume struct {
	Name        string  `json:"name" form:"name"`
	Path        string  `json:"path,omitempty" form:"path,omitempty"`
	Vid         int     `json:"vid,omitempty" form:"vid,omitempty"`
	Dynamic     bool    `json:"dynamic,omitempty" form:"dynamic,omitempty"`
	Capacity    float64 `json:"capacity,omitempty" form:"capacity,omitempty"`
	Usage       float64 `json:"usage,omitempty" form:"usage,omitempty"`
	CreatedAt   string  `json:"createdAt,omitempty" form:"createdAt,omitempty"`
	ObjectCount int     `json:"object_count,omitempty" form:"object_count,omitempty"`
}

/* fields and ptr fields for "volume" struct helper methods*/

// Fields returns a slice containing the values of the Volume struct fields,
// in a specific order suitable for database operations or serialization.
// The returned slice includes all fields, including IDs and metadata.
func (v Volume) Fields() []any {
	return []any{v.Vid, v.Name, v.Path, v.Dynamic, v.Capacity, v.Usage, v.CreatedAt}
}

// PtrFields returns a slice of pointers to the Volume struct fields,
// in a specific order. This is useful for scanning database rows directly
// into the struct fields or for generic update operations.
func (v *Volume) PtrFields() []any {
	return []any{&v.Vid, &v.Name, &v.Path, &v.Dynamic, &v.Capacity, &v.Usage, &v.CreatedAt}
}

// FieldsNoID returns a slice containing the values of the Volume struct fields,
// excluding the primary ID (Rid). This is typically used for insert or update
// operations where the ID is auto-generated or not required.
func (v Volume) FieldsNoID() []any {
	return []any{v.Name, v.Path, v.Dynamic, v.Capacity, v.Usage, v.CreatedAt}
}

// PtrFieldsNoID returns a slice of pointers to the Volume struct fields,
// excluding the primary ID (Rid). This is typically used for insert or update
// operations where the ID is auto-generated or not required.
func (v *Volume) PtrFieldsNoID() []any {
	return []any{&v.Name, &v.Path, &v.Dynamic, &v.Capacity, &v.Usage, &v.CreatedAt}
}

// Validate method verifies the volume object is allowed, given the rules
func (v *Volume) Validate(maxCapacity, fallbackCapacity float64, plusChars string) error {
	// name should be specific
	// path should exist
	// vid should exist

	// for starters, check just the name
	if len(v.Name) < 4 {
		return fmt.Errorf("volume name cannot be less or equal than 3 characters")
	}

	if len(v.Name) > 63 {
		return fmt.Errorf("volume name too large: max 63 chars")
	}

	if !IsAlphanumericPlus(v.Name, plusChars) {
		return fmt.Errorf("volume name must contain only alphanumeric characters and '.' or '-'")
	}

	if maxCapacity > 0 && v.Capacity > maxCapacity {
		v.Capacity = fallbackCapacity
	}

	return nil
}

/* a struct to represent each user volume relationship */

// UserVolume struct describes the "chunk" of a user upon a volume
type UserVolume struct {
	UpdatedAt string  `json:"updated_at"`
	Vid       int     `json:"vid"`
	UID       int     `json:"uid"`
	Usage     float64 `json:"usage"`
	Quota     float64 `json:"quota"`
}

// PtrFields returns a slice of pointers to the UserVolume struct fields,
// in a specific order. This is useful for scanning database rows directly
// into the struct fields or for generic update operations.
func (uv *UserVolume) PtrFields() []any {
	return []any{&uv.Vid, &uv.UID, &uv.Usage, &uv.Quota, &uv.UpdatedAt}
}

// Fields returns a slice containing the values of the UserVolume struct fields,
// in a specific order suitable for database operations or serialization.
// The returned slice includes all fields, including IDs and metadata.
func (uv *UserVolume) Fields() []any {
	return []any{uv.Vid, uv.UID, uv.Usage, uv.Quota, uv.UpdatedAt}
}

/* a struct to represent a volume claim by a group*/

// GroupVolume struct describes the "chunk" of volume a group inflicts upon it
type GroupVolume struct {
	UpdatedAt string  `json:"updated_at"`
	Vid       int     `json:"vid"`
	Gid       int     `json:"gid"`
	Usage     float64 `json:"usage"`
	Quota     float64 `json:"quota"`
}

// PtrFields returns a slice of pointers to the GroupVolume struct fields,
// in a specific order. This is useful for scanning database rows directly
// into the struct fields or for generic update operations.
func (gv *GroupVolume) PtrFields() []any {
	return []any{&gv.Vid, &gv.Gid, &gv.Usage, &gv.Quota, &gv.UpdatedAt}
}

// Fields returns a slice containing the values of the GroupVolume struct fields,
// in a specific order suitable for database operations or serialization.
// The returned slice includes all fields, including IDs and metadata.
func (gv *GroupVolume) Fields() []any {
	return []any{gv.Vid, gv.Gid, gv.Usage, gv.Quota, gv.UpdatedAt}
}

/* Header information the service needs to handle requests.
*  given uid, gids, volume_name/vid, resourceName
*  (who the user is and in which groups he belongs to and what he seeks)
* */

// AccessClaim carries user and volume access context.
type AccessClaim struct {
	// UID is the user ID.
	// @example 1001
	UID string `json:"userID" example:"1001"`

	// Gids is a comma-separated string of group IDs.
	// @example 2001,2002
	Gids string `json:"groupIDs" example:"2001,2002"`

	// Vid is the volume ID.
	// @example 1
	Vid string `json:"volume_id" example:"1"`

	// Vname is the name of the volume.
	// @example default
	Vname string `json:"volume_name" example:"default"`

	// HasKeyword indicates if the user used a special keyword in the request.
	HasKeyword bool `json:"haskeyword,omitempty"`

	// Target is the requested resource or path.
	// @example /data/reports
	Target string `json:"target" example:"/data/reports"`
}

// Filter method cleans the object data, mostly strings
func (ac *AccessClaim) Filter() AccessClaim {
	/* can enrich this method */
	return AccessClaim{
		UID:    strings.TrimSpace(ac.UID),
		Gids:   strings.TrimSpace(ac.Gids),
		Target: strings.TrimSpace(ac.Target),
		Vid:    ac.Vid,
	}
}

// Validate method verifies the object is within limits
func (ac *AccessClaim) Validate() error {
	if ac.UID == "" && ac.Gids == "" {
		return fmt.Errorf("cannot have empty ids")
	}
	if ac.Target == "" {
		return fmt.Errorf("cannot have empty target")
	}

	return nil
}

// User represents a system user.
// @Description Contains user metadata, credentials, and group memberships.
type User struct {
	// Username is the unique login name of the user.
	// @example johndoe
	Username string `json:"username" example:"johndoe"`

	// Info is optional user metadata or notes.
	Info string `json:"info,omitempty" example:"researcher in group A"`

	// Home is the user’s home directory path.
	Home string `json:"home,omitempty" example:"/home/johndoe"`

	// Shell is the user’s default shell.
	Shell string `json:"shell,omitempty" example:"/bin/bash"`

	// Password contains the user’s password settings and hash.
	Password Password `json:"password"`

	// Groups is a list of groups the user belongs to.
	Groups []Group `json:"groups,omitempty"`

	// UID is the user’s numeric ID.
	UID int `json:"uid,omitempty"`

	// Pgroup is the user’s primary group ID.
	Pgroup int `json:"pgroup,omitempty"`
}

// ToString method formats and returns the permission object to a string
func (u *User) ToString() string {
	return fmt.Sprintf(`
		Name: %s, UID: %d
	`, u.Username, u.UID,
	)
}

// UsersToString function is similar to ToString method but works for a slice of User objects
func UsersToString(users []User) string {
	var res []string

	for _, user := range users {
		res = append(res, user.Username)
	}

	return strings.Join(res, ",")
}

// Password represents user password data and policies.
type Password struct {
	// Hashpass is the hashed password.
	// @example $2a$10$7s5YfF7...
	Hashpass string `json:"hashpass" example:"$2a$10$7s5YfF7..."`

	// LastPasswordChange is the last time the password was changed.
	LastPasswordChange string `json:"lastPasswordChange,omitempty"`

	MinimumPasswordAge string `json:"minimumPasswordAge,omitempty"`
	MaximumPasswordAge string `json:"maxiumPasswordAge,omitempty"`
	WarningPeriod      string `json:"warningPeriod,omitempty"`
	InactivityPeriod   string `json:"inactivityPeriod,omitempty"`
	ExpirationDate     string `json:"expirationDate,omitempty"`
}

/* check password fields for allowed values...*/

// ValidatePassword method sanitizes the password object and checks if it is within limits
func (p *Password) ValidatePassword() error {
	pass := p.Hashpass

	// Validate Password Length
	if len(pass) < 8 {
		return NewError("password length '%d' is too short: minimum required length is 8 characters", len(pass))
	}

	// Validate Hashpass
	if pass == "" {
		return NewError("hashpass cannot be empty")
	}

	var hasUpper bool
	var hasDigit bool

	for _, r := range pass {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}

	if !hasUpper {
		return NewError("password must contain at least one uppercase letter")
	}

	if !hasDigit {
		return NewError("password must contain at least one digit")
	}

	return nil
}

// Group represents a group of users in the system.
type Group struct {
	// Groupname is the name of the group.
	// @example researchers
	Groupname string `json:"groupname" example:"researchers"`

	// Users is an optional list of users in the group.
	Users []User `json:"users,omitempty" swaggerignore:"true"`

	// Gid is the numeric group ID.
	// @example 3001
	Gid int `json:"gid" example:"3001"`
}

// PtrFields returns a slice of pointers to the Resource struct fields,
// in a specific order. This is useful for scanning database rows directly
// into the struct fields or for generic update operations.
func (g *Group) PtrFields() []any {
	return []any{&g.Groupname, &g.Gid}
}

// ToString method formats and returns the permission object to a string
func (g *Group) ToString() string {
	return fmt.Sprintf("%v", g.Groupname)
}

// GroupsToString function is similar to ToString method but works for a slice of Group objects and affects only names
func GroupsToString(groups []Group) string {
	var res []string

	for _, group := range groups {
		res = append(res, group.ToString())
	}

	return strings.Join(res, ",")
}

// GidsToString function is similar to ToString method but works for a slice of Group objects but affects only group ids
func GidsToString(groups []Group) string {
	var res []string
	for _, group := range groups {
		res = append(res, strconv.Itoa(group.Gid))
	}
	return strings.Join(res, ",")
}

// Job struct defines all the data required and optional for executing a Job.
type Job struct {
	Jid int64 `json:"jid,omitempty" form:"jid"`
	UID int   `json:"uid" form:"uid"`

	Parallelism int `json:"parallelism,omitempty" form:"parallelism"`
	Priority    int `json:"priority,omitempty" form:"priority"`

	MemoryRequest string `json:"memory_request,omitempty" form:"memory_request"`
	CPURequest    string `json:"cpu_request,omitempty" form:"cpu_request"`
	MemoryLimit   string `json:"memory_limit,omitempty" form:"memory_limit"`
	CPULimit      string `json:"cpu_limit,omitempty" form:"cpu_limit"`

	EphimeralStorageRequest string `json:"ephimeral_storage_request,omitempty" form:"ephimeral_storage_request"`
	EphimeralStorageLimit   string `json:"ephimeral_storage_limit,omitempty" form:"ephimeral_storage_limit"`

	Description string  `json:"description,omitempty" form:"description"`
	Duration    float64 `json:"duration,omitempty" form:"duration"`

	Input   string `json:"input" form:"input"`
	Output  string `json:"output" form:"output"`
	Timeout int    `json:"timeout,omitempty" form:"timeout"` // in minutes

	Env map[string]string `json:"env,omitempty"`

	// perhaps unecessary
	InputFormat  string `json:"input_format,omitempty"`
	OutputFormat string `json:"output_format,omitempty"`

	Logic        string   `json:"logic" form:"logic"`
	LogicBody    string   `json:"logic_body" form:"logic_body"`
	LogicHeaders string   `json:"logic_headers,omitempty" form:"logic_headers"`
	Params       []string `json:"params,omitempty" form:"params"`

	Status      string `json:"status,omitempty" form:"status"`
	Completed   bool   `json:"completed,omitempty" form:"completed"`
	CompletedAt string `json:"completed_at,omitempty" form:"completed_at"`
	CreatedAt   string `json:"createdAt,omitempty" form:"createdAt"`
}

// ValidateForm method sanitizes and checks if the given Job object is within limits
func (j *Job) ValidateForm(maxCPU, maxMem, maxStorage, maxParal, maxTimeout, maxChars int64) error {
	// Validate
	if j.Description != "" && !IsValidUTF8String(j.Description) {
		return fmt.Errorf("description must contain valid characters")
	}

	if j.LogicHeaders != "" && !IsValidUTF8String(j.LogicHeaders) {
		return fmt.Errorf("headers must contain valid characters")
	}

	if j.Logic == "" {
		return fmt.Errorf("must provide logic")
	}

	if !IsValidUTF8String(j.Logic) {
		return fmt.Errorf("logic must contain valid characters")
	}

	if len(j.LogicBody) == 0 {
		return fmt.Errorf("must provide logic")
	}

	if int64(len(j.LogicBody)) > maxChars {
		return fmt.Errorf("logic_body exceeds max length of %v characters", maxChars)
	}

	if j.Params != nil && !IsValidUTF8String(strings.Join(j.Params, " ")) {
		return fmt.Errorf("headers must contain valid characters")
	}

	if j.Input == "" {
		return fmt.Errorf("must provide input")
	}

	if j.Output == "" {
		return fmt.Errorf("must provide output")
	}
	// Validate paths
	if !IsValidPath(j.Input) || !IsValidPath(j.Output) {
		return fmt.Errorf("input/output paths contain invalid characters")
	}

	if len(j.Description) > 150 {
		return fmt.Errorf("description must be less or equal than 150 characters")
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
	storageReq, err := parseGi(j.EphimeralStorageRequest)
	if err != nil || storageReq > float64(maxStorage) {
		return fmt.Errorf("ephemeral_storage_request must be less than %vGi", maxStorage)
	}

	storageLim, err := parseGi(j.EphimeralStorageLimit)
	if err != nil || storageLim > float64(maxStorage) {
		return fmt.Errorf("ephemeral_storage_limit must be less than %vGi", maxStorage)
	}

	// sanitize
	j.Input = strings.TrimSpace(j.Input)
	j.Output = strings.TrimSpace(j.Output)

	if p := strings.Split(j.Output, "/"); len(p) != 2 || p[1] == "" {
		r, err := generateRandomString(16)
		if err != nil {
			return fmt.Errorf("failed to generate output name, must provide an output object name")
		}
		j.Output = j.Output + r
	}

	j.MemoryRequest = appendIfMissing(strings.TrimSpace(j.MemoryRequest), "Mi")
	j.MemoryLimit = appendIfMissing(strings.TrimSpace(j.MemoryLimit), "Mi")

	j.EphimeralStorageRequest = appendIfMissing(strings.TrimSpace(j.EphimeralStorageRequest), "Gi")
	j.EphimeralStorageLimit = appendIfMissing(strings.TrimSpace(j.EphimeralStorageLimit), "Gi")

	j.CPURequest = strings.TrimSpace(j.CPURequest)
	j.CPULimit = strings.TrimSpace(j.CPULimit)

	return nil
}

// PtrFields returns a slice of pointers to the Job struct fields,
// in a specific order. This is useful for scanning database rows directly
// into the struct fields or for generic update operations.
func (j *Job) PtrFields() []any {
	return []any{&j.Jid, &j.UID, &j.Description, &j.Duration, &j.Input, &j.InputFormat, &j.Output, &j.OutputFormat, &j.Logic, &j.LogicBody, &j.LogicHeaders, &j.Params, &j.Status, &j.Completed, &j.CompletedAt, &j.CreatedAt, &j.Parallelism, &j.Priority, &j.MemoryRequest, &j.CPURequest, &j.MemoryLimit, &j.CPULimit, &j.EphimeralStorageRequest, &j.EphimeralStorageLimit}
}

// Fields returns a slice containing the values of the Job struct fields,
// in a specific order suitable for database operations or serialization.
// The returned slice includes all fields, including IDs and metadata.
func (j *Job) Fields() []any {
	return []any{j.Jid, j.UID, j.Description, j.Duration, j.Input, j.InputFormat, j.Output, j.OutputFormat, j.Logic, j.LogicBody, j.LogicHeaders, j.Params, j.Status, j.Completed, j.CompletedAt, j.CreatedAt, j.Parallelism, j.Priority, j.MemoryRequest, j.CPURequest, j.MemoryLimit, j.CPULimit, j.EphimeralStorageRequest, j.EphimeralStorageLimit}
}

// PtrFieldsNoID returns a slice of pointers to the Job struct fields,
// excluding the primary ID (Rid). This is typically used for insert or update
// operations where the ID is auto-generated or not required.
func (j *Job) PtrFieldsNoID() []any {
	return []any{&j.UID, &j.Description, &j.Duration, &j.Input, &j.InputFormat, &j.Output, &j.OutputFormat, &j.Logic, &j.LogicBody, &j.LogicHeaders, &j.Params, &j.Status, &j.Completed, &j.CompletedAt, &j.CreatedAt, &j.Parallelism, &j.Priority, &j.MemoryRequest, &j.CPURequest, &j.MemoryLimit, &j.CPULimit, &j.EphimeralStorageRequest, &j.EphimeralStorageLimit}
}

// FieldsNoID returns a slice containing the values of the Job struct fields,
// excluding the primary ID (Rid). This is typically used for insert or update
// operations where the ID is auto-generated or not required.
func (j *Job) FieldsNoID() []any {
	return []any{j.UID, j.Description, j.Duration, j.Input, j.InputFormat, j.Output, j.OutputFormat, j.Logic, j.LogicBody, j.LogicHeaders, j.Params, j.Status, j.Completed, j.CompletedAt, j.CreatedAt, j.Parallelism, j.Priority, j.MemoryRequest, j.CPURequest, j.MemoryLimit, j.CPULimit, j.EphimeralStorageRequest, j.EphimeralStorageLimit}
}

// APIResponse aims to unite the type of responses the microservices return, bricking the "Response Model"
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
	AuthorID    int    `json:"author_id,omitempty"`
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
