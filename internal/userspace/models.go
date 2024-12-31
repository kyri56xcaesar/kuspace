package userspace

/*
* Core structures for the Userspace API and their methods.
*
* perhaps some utility functions as well.
*
* */
import (
	"fmt"
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
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Created_at  string      `json:"created_at"`
	Updated_at  string      `json:"updated_at"`
	Accessed_at string      `json:"accessed_at"`
	Perms       Permissions `json:"perms"`
	Rid         int         `json:"rid"`
	Uid         int         `json:"uid"` // as in user id (owner)
	Vid         int         `json:"vid"`
	Gid         int         `json:"gid"` // as in group id
	Pid         int         `json:"pid"` // as in parent id
	Size        int         `json:"size"`
	Links       int         `json:"links"`
}

type Permissions struct {
	Owner PermTriplet `json:"owner"`
	Group PermTriplet `json:"group"`
	Other PermTriplet `json:"other"`
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
	Path     string `json:"path"`
	Vid      int    `json:"vid"`
	Capacity int    `json:"capacity"`
	Usage    int    `json:"usage"`
}

/* representative utility methods of the above structures */
/* fields and ptrs fields for "resource" struct helper methods*/
func (r *Resource) Fields() []any {
	return []any{r.Rid, r.Uid, r.Vid, r.Gid, r.Pid, r.Size, r.Perms, r.Name, r.Type, r.Created_at, r.Updated_at}
}

func (r *Resource) PtrFields() []any {
	return []any{&r.Rid, &r.Uid, &r.Vid, &r.Gid, &r.Pid, &r.Size, &r.Perms, &r.Name, &r.Type, &r.Created_at, &r.Updated_at}
}

func (r *Resource) FieldsNoId() []any {
	return []any{r.Uid, r.Vid, r.Gid, r.Pid, r.Size, r.Perms, r.Name, r.Type, r.Created_at, r.Updated_at}
}

func (r *Resource) PtrFieldsNoId() []any {
	return []any{&r.Uid, &r.Vid, &r.Gid, &r.Pid, &r.Size, &r.Perms, &r.Name, &r.Type, &r.Created_at, &r.Updated_at}
}

/* fields and ptr fields for "volume" struct helper methods*/
func (v *Volume) Fields() []any {
	return []any{v.Vid, v.Path, v.Capacity, v.Usage}
}

func (v *Volume) PtrFields() []any {
	return []any{&v.Vid, &v.Path, &v.Capacity, &v.Usage}
}

func (v *Volume) FieldsNoId() []any {
	return []any{v.Path, v.Capacity, v.Usage}
}

func (v *Volume) PtrFieldsNoId() []any {
	return []any{&v.Path, &v.Capacity, &v.Usage}
}

/* This service will handle requests
*  according to given uid and his gids
*  (who the user is and in which groups he belongs to)
* */
type AccessClaim struct {
	Uid  string `json:"user_id"`
	Gids string `json:"group_ids"`

	Target string `json:"target"`
}

func (ac *AccessClaim) filter() AccessClaim {
	/* can enrich this method */
	return AccessClaim{
		Uid:    strings.TrimSpace(ac.Uid),
		Gids:   strings.TrimSpace(ac.Gids),
		Target: strings.TrimSpace(ac.Target),
	}
}

func (ac *AccessClaim) validate() error {
	if ac.Uid == "" && ac.Gids == "" {
		return fmt.Errorf("cannot have empty ids")
	}
	if ac.Target == "" {
		return fmt.Errorf("cannot have empty target")
	}

	return nil
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

	/* check ownership
	* if true, can exit prematurely
	* */
	if uid, err := strconv.Atoi(userInfo.Uid); err != nil {
		log.Printf("error atoing user id")
		return false
	} else if uid == resource.Uid {
		return resource.Perms.Owner.Read
	}

	/* check groups */
	gids := strings.Split(userInfo.Gids, ",")

	for _, gid := range gids {
		if igid, err := strconv.Atoi(gid); err != nil {
			log.Printf("error atoing group id")
			return false
		} else if igid == resource.Gid {
			return resource.Perms.Group.Read
		}
	}
	/* check others */
	return resource.Perms.Other.Read
}

/* similar as above just for write access*/
func (resource *Resource) HasWriteAccess(userInfo AccessClaim) bool {
	/* parse permissions
	*  rwx     rwx     rwx       644, (unix inode metadata)
	*   |       |       |
	* owner   group   others
	* */

	/* check ownership
	* if true, can exit prematurely
	* */
	if uid, err := strconv.Atoi(userInfo.Uid); err != nil {
		log.Printf("error atoing user id")
		return false
	} else if uid == resource.Uid {
		return resource.Perms.Owner.Write
	}

	/* check groups */
	gids := strings.Split(userInfo.Gids, ",")

	for _, gid := range gids {
		if igid, err := strconv.Atoi(gid); err != nil {
			log.Printf("error atoing group id")
			return false
		} else if igid == resource.Gid {
			return resource.Perms.Group.Write
		}
	}
	/* check others */
	return resource.Perms.Other.Write
}

/* execution access is somewhat trivial at this point, perhaps it can be used in the future*/
func (resource *Resource) HasExecutionAccess(userInfo AccessClaim) bool {
	return false
}

/* generic helpers*/
func toSnakeCase(input string) string {
	var output []rune
	for i, r := range input {
		if i > 0 && r >= 'A' && r <= 'Z' {
			output = append(output, '_')
		}
		output = append(output, r)
	}

	return strings.ToLower(string(output))
}
