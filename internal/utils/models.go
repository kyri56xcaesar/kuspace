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
)

/*
* Resource struct
*
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

	Created_at  string `json:"created_at,omitempty"`
	Updated_at  string `json:"updated_at,omitempty"`
	Accessed_at string `json:"accessed_at,omitempty"`

	Perms string `json:"perms,omitempty"`

	Rid int `json:"rid,omitempty"`
	Uid int `json:"uid,omitempty"` // as in user id (owner)
	Gid int `json:"gid,omitempty"` // as in group id

	Size  int64 `json:"size,omitempty"`
	Links int   `json:"links,omitempty"`

	Vid   int    `json:"vid,omitempty"`
	Vname string `json:"vname,omitempty"`

	Reader io.Reader `json:"reader,omitempty"`
}

/* representative utility methods of the above structures */
/* fields and ptrs fields for "resource" struct helper methods*/
func (r *Resource) Fields() []any {
	return []any{r.Rid, r.Uid, r.Gid, r.Vid, r.Vname, r.Size, r.Links, r.Perms, r.Name, r.Path, r.Type, r.Created_at, r.Updated_at, r.Accessed_at}
}

func (r *Resource) PtrFields() []any {
	return []any{&r.Rid, &r.Uid, &r.Gid, &r.Vid, &r.Vname, &r.Size, &r.Links, &r.Perms, &r.Name, &r.Path, &r.Type, &r.Created_at, &r.Updated_at, &r.Accessed_at}
}

func (r *Resource) FieldsNoId() []any {
	return []any{r.Uid, r.Gid, r.Vid, r.Vname, r.Size, r.Links, r.Perms, r.Name, r.Path, r.Type, r.Created_at, r.Updated_at, r.Accessed_at}
}

func (r *Resource) PtrFieldsNoId() []any {
	return []any{&r.Uid, &r.Gid, &r.Vid, &r.Vname, &r.Size, &r.Links, &r.Perms, &r.Name, &r.Path, &r.Type, &r.Created_at, &r.Updated_at, &r.Accessed_at}
}

/* this method belongs to the Resource objects
*  and checks if a user claim it allowed to acccess it
*
*  -> authorization.
* */
func (resource *Resource) HasAccess(userInfo AccessClaim) bool {
	/* parse permissions
	*  rwx     rwx     rwx       644, (unix inode metadata)
	*   |       |       |
	* owner   group   others
	* */

	var perm Permissions
	perm.fillFromStr(resource.Perms)

	/* check ownership
	* if true, can exit prematurely
	* */
	if uid, err := strconv.Atoi(userInfo.Uid); err != nil {
		log.Printf("error atoing user id")
		return false
	} else if uid == resource.Uid {
		return perm.Owner.Read
	}

	/* check groups */
	gids := strings.Split(userInfo.Gids, ",")

	for _, gid := range gids {
		if igid, err := strconv.Atoi(gid); err != nil {
			log.Printf("error atoing group id")
			return false
		} else if igid == resource.Gid {
			return perm.Group.Read
		}
	}
	/* check others */
	return perm.Other.Read
}

/* similar as above just for write access*/
func (resource *Resource) HasWriteAccess(userInfo AccessClaim) bool {
	/* parse permissions
	*  rwx     rwx     rwx       644, (unix inode metadata)
	*   |       |       |
	* owner   group   others
	* */
	var perm Permissions
	perm.fillFromStr(resource.Perms)

	/* check ownership
	* if true, can exit prematurely
	* */
	if uid, err := strconv.Atoi(userInfo.Uid); err != nil {
		log.Printf("error atoing user id")
		return false
	} else if uid == resource.Uid {
		return perm.Owner.Write
	}

	/* check groups */
	gids := strings.Split(userInfo.Gids, ",")

	for _, gid := range gids {
		if igid, err := strconv.Atoi(gid); err != nil {
			log.Printf("error atoing group id")
			return false
		} else if igid == resource.Gid {
			return perm.Group.Write
		}
	}
	/* check others */
	return perm.Other.Write
}

/* execution access is somewhat trivial at this point, perhaps it can be used in the future*/
func (resource *Resource) HasExecutionAccess(userInfo AccessClaim) bool {
	return false
}

func (resource *Resource) IsOwner(ac AccessClaim) bool {
	int_uid, err := strconv.Atoi(ac.Uid)
	if err != nil {
		log.Printf("failed to atoi access_claim")
		return false
	}
	return resource.Uid == int_uid
}

type Permissions struct {
	Representation string      `json:"perms"`
	Owner          PermTriplet `json:"owner"`
	Group          PermTriplet `json:"group"`
	Other          PermTriplet `json:"other"`
}

/* this method goal is from the given argument "representation" which
* looks like "r-x---r--" (or w.e) to transform into the boolean
* */
func (p *Permissions) fillFromStr(representation string) error {
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

func (p *Permissions) ToString() string {
	return fmt.Sprintf("%s%s%s", p.Owner.ToString(), p.Group.ToString(), p.Other.ToString())
}

type PermTriplet struct {
	Title   string `json:"title"`
	Read    bool   `json:"read"`
	Write   bool   `json:"write"`
	Execute bool   `json:"execute"`
}

func (pt *PermTriplet) ToString() string {
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
type Volume struct {
	Name        string  `json:"name" form:"name"`
	Path        string  `json:"path,omitempty" form:"path,omitempty"`
	Vid         int     `json:"vid,omitempty" form:"vid,omitempty"`
	Dynamic     bool    `json:"dynamic,omitempty" form:"dynamic,omitempty"`
	Capacity    float64 `json:"capacity,omitempty" form:"capacity,omitempty"`
	Usage       float64 `json:"usage,omitempty" form:"usage,omitempty"`
	CreatedAt   string  `json:"created_at,omitempty" form:"created_at,omitempty"`
	ObjectCount int     `json:"object_count,omitempty" form:"object_count,omitempty"`
}

/* fields and ptr fields for "volume" struct helper methods*/
func (v *Volume) Fields() []any {
	return []any{v.Vid, v.Name, v.Path, v.Dynamic, v.Capacity, v.Usage, v.CreatedAt}
}

func (v *Volume) PtrFields() []any {
	return []any{&v.Vid, &v.Name, &v.Path, &v.Dynamic, &v.Capacity, &v.Usage, &v.CreatedAt}
}

func (v *Volume) FieldsNoId() []any {
	return []any{v.Name, v.Path, v.Dynamic, v.Capacity, v.Usage, v.CreatedAt}
}

func (v *Volume) PtrFieldsNoId() []any {
	return []any{&v.Name, &v.Path, &v.Dynamic, &v.Capacity, &v.Usage, &v.CreatedAt}
}

func (v *Volume) Validate(max_capacity, fallback_capacity float64) error {
	// name should be specific
	// path should exist
	// vid should exist

	// for starters, check just the name
	if len(v.Name) > 63 {
		return fmt.Errorf("volume name too large: max 63 chars")
	}
	if HasInvalidCharacters(v.Name, "^*|\\/&\"'.,;") {
		return fmt.Errorf("invalid characters in the name. (^*|\\/&\"_,;) not allowed")
	}

	if max_capacity > 0 && v.Capacity > max_capacity {
		v.Capacity = fallback_capacity
	}

	return nil
}

/* a struct to represent each user volume relationship */
type UserVolume struct {
	Updated_at string  `json:"updated_at"`
	Vid        int     `json:"vid"`
	Uid        int     `json:"uid"`
	Usage      float64 `json:"usage"`
	Quota      float64 `json:"quota"`
}

func (uv *UserVolume) PtrFields() []any {
	return []any{&uv.Vid, &uv.Uid, &uv.Usage, &uv.Quota, &uv.Updated_at}
}

func (uv *UserVolume) Fields() []any {
	return []any{uv.Vid, uv.Uid, uv.Usage, uv.Quota, uv.Updated_at}
}

/* a struct to represent a volume claim by a group*/
type GroupVolume struct {
	Updated_at string  `json:"updated_at"`
	Vid        int     `json:"vid"`
	Gid        int     `json:"gid"`
	Usage      float64 `json:"usage"`
	Quota      float64 `json:"quota"`
}

func (gv *GroupVolume) PtrFields() []any {
	return []any{&gv.Vid, &gv.Gid, &gv.Usage, &gv.Quota, &gv.Updated_at}
}

func (gv *GroupVolume) Fields() []any {
	return []any{gv.Vid, gv.Gid, gv.Usage, gv.Quota, gv.Updated_at}
}

/* Header information the service needs to handle requests.
*  given uid, gids, volume_name/vid, resource_name
*  (who the user is and in which groups he belongs to and what he seeks)
* */
// AccessClaim carries user and volume access context.
type AccessClaim struct {
	// Uid is the user ID.
	// @example 1001
	Uid string `json:"user_id" example:"1001"`

	// Gids is a comma-separated string of group IDs.
	// @example 2001,2002
	Gids string `json:"group_ids" example:"2001,2002"`

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

func (ac *AccessClaim) Filter() AccessClaim {
	/* can enrich this method */
	return AccessClaim{
		Uid:    strings.TrimSpace(ac.Uid),
		Gids:   strings.TrimSpace(ac.Gids),
		Target: strings.TrimSpace(ac.Target),
		Vid:    ac.Vid,
	}
}

func (ac *AccessClaim) Validate() error {
	if ac.Uid == "" && ac.Gids == "" {
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

	// Uid is the user’s numeric ID.
	Uid int `json:"uid,omitempty" example:"1001"`

	// Pgroup is the user’s primary group ID.
	Pgroup int `json:"pgroup,omitempty" example:"2001"`
}

func (u *User) ToString() string {
	return fmt.Sprintf(`
		Name: %s, Uid: %d
	`, u.Username, u.Uid,
	)
}

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
func (p *Password) ValidatePassword() error {
	// Validate Password Length
	if len(p.Hashpass) < 4 {
		return NewError("password length '%d' is too short: minimum required length is 4 characters", len(p.Hashpass))
	}

	// Validate Hashpass
	if p.Hashpass == "" {
		return NewError("hashpass cannot be empty")
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

func (g *Group) PtrFields() []any {
	return []any{&g.Groupname, &g.Gid}
}

func (g *Group) ToString() string {
	return fmt.Sprintf("%v", g.Groupname)
}

func GroupsToString(groups []Group) string {
	var res []string

	for _, group := range groups {
		res = append(res, group.ToString())
	}

	return strings.Join(res, ",")
}

func GidsToString(groups []Group) string {
	var res []string
	for _, group := range groups {
		res = append(res, strconv.Itoa(group.Gid))
	}
	return strings.Join(res, ",")
}

type Job struct {
	Jid int64 `json:"jid,omitempty"`
	Uid int   `json:"uid"`

	Parallelism int `json:"parallelism,omitempty"`
	Priority    int `json:"priority,omitempty"`

	MemoryRequest string `json:"memory_request,omitempty"`
	CpuRequest    string `json:"cpu_request,omitempty"`
	MemoryLimit   string `json:"memory_limit,omitempty"`
	CpuLimit      string `json:"cpu_limit,omitempty"`

	EphimeralStorageRequest string `json:"ephimeral_storage_request,omitempty"`
	EphimeralStorageLimit   string `json:"ephimeral_storage_limit,omitempty"`

	Description string  `json:"description,omitempty"`
	Duration    float64 `json:"duration,omitempty"`

	Input   string `json:"input"`
	Output  string `json:"output"`
	Timeout int    `json:"timeout,omitempty"` // in minutes

	Env map[string]string `json:"env,omitempty"`

	// perhaps unecessary
	InputFormat  string `json:"input_format,omitempty"`
	OutputFormat string `json:"output_format,omitempty"`

	Logic        string   `json:"logic"`
	LogicBody    string   `json:"logic_body"`
	LogicHeaders string   `json:"logic_headers,omitempty"`
	Params       []string `json:"params,omitempty"`

	Status       string `json:"status,omitempty"`
	Completed    bool   `json:"completed,omitempty"`
	Completed_at string `json:"completed_at,omitempty"`
	Created_at   string `json:"created_at,omitempty"`
}

func (j *Job) PtrFields() []any {
	return []any{&j.Jid, &j.Uid, &j.Description, &j.Duration, &j.Input, &j.InputFormat, &j.Output, &j.OutputFormat, &j.Logic, &j.LogicBody, &j.LogicHeaders, &j.Params, &j.Status, &j.Completed, &j.Completed_at, &j.Created_at, &j.Parallelism, &j.Priority, &j.MemoryRequest, &j.CpuRequest, &j.MemoryLimit, &j.CpuLimit, &j.EphimeralStorageRequest, &j.EphimeralStorageLimit}
}

func (j *Job) Fields() []any {
	return []any{j.Jid, j.Uid, j.Description, j.Duration, j.Input, j.InputFormat, j.Output, j.OutputFormat, j.Logic, j.LogicBody, j.LogicHeaders, j.Params, j.Status, j.Completed, j.Completed_at, j.Created_at, j.Parallelism, j.Priority, j.MemoryRequest, j.CpuRequest, j.MemoryLimit, j.CpuLimit, j.EphimeralStorageRequest, j.EphimeralStorageLimit}
}

func (j *Job) PtrFieldsNoId() []any {
	return []any{&j.Uid, &j.Description, &j.Duration, &j.Input, &j.InputFormat, &j.Output, &j.OutputFormat, &j.Logic, &j.LogicBody, &j.LogicHeaders, &j.Params, &j.Status, &j.Completed, &j.Completed_at, &j.Created_at, &j.Parallelism, &j.Priority, &j.MemoryRequest, &j.CpuRequest, &j.MemoryLimit, &j.CpuLimit, &j.EphimeralStorageRequest, &j.EphimeralStorageLimit}
}

func (j *Job) FieldsNoId() []any {
	return []any{j.Uid, j.Description, j.Duration, j.Input, j.InputFormat, j.Output, j.OutputFormat, j.Logic, j.LogicBody, j.LogicHeaders, j.Params, j.Status, j.Completed, j.Completed_at, j.Created_at, j.Parallelism, j.Priority, j.MemoryRequest, j.CpuRequest, j.MemoryLimit, j.CpuLimit, j.EphimeralStorageRequest, j.EphimeralStorageLimit}
}

type APIResponse[T any] struct {
	Status  string `json:"status"`  // e.g., "success", "error"
	Message string `json:"message"` // e.g., "Operation successful"
	Data    T      `json:"data"`    // Any data payload
}
