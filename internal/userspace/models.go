package userspace

import (
	"strings"
)

type Resource struct {
	Rid        int    `json:"rid"`
	Uid        int    `json:"uid"` // as in user id (owner)
	Vid        int    `json:"vid"`
	Gid        int    `json:"gid"` // as in group id
	Pid        int    `json:"pid"` // as in parent id
	Size       int    `json:"size"`
	Perms      string `json:"perms"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Created_at string `json:"created_at"`
	Updated_at string `json:"updated_at"`
}

type Volume struct {
	Vid      int    `json:"vid"`
	Capacity int    `json:"capacity"`
	Usage    int    `json:"usage"`
	Path     string `json:"path"`
}

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
