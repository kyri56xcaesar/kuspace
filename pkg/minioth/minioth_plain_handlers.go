package minioth

import (
	"bufio"
	"fmt"
	ut "kyri56xcaesar/myThesis/internal/utils"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
)

const (
	MINIOTH_PASSWD   string = "data/plain/minioth/mpasswd"
	MINIOTH_GROUP    string = "data/plain/minioth/mgroup"
	MINIOTH_SHADOW   string = "data/plain/minioth/mshadow"
	PLACEHOLDER_PASS string = "bcrypted_password_hash" // what is shown in passwd for password
	DEL              string = ":"                      //delimitter
	// username:password ref:uuid:guid:info:home:shell
	ENTRY_MPASSWD_FORMAT string = "%s" + DEL + "%s" + DEL + "%v" + DEL + "%v" + DEL + "%s" + DEL + "%s" + DEL + "%s\n"
	// username:hashed_pass:last_password_change:min_password_age:max_password_age:warning_period:inactivity_period:expiration_date: (bunch of times)
	ENTRY_MSHADOW_FORMAT string = "%s" + DEL + "%s" + DEL + "%s" + DEL + "%s" + DEL + "%s" + DEL + "%s" + DEL + "%s" + DEL + "%s" + DEL + "%v\n"
	// groupname:group_id:users
	ENTRY_MGROUP_FORMAT string = "%s" + DEL + "%v" + DEL + "%v\n"
)

var (
	writeLock            = sync.Mutex{}
	current_admin_id int = 0
	current_mod_id   int = 100
	current_user_id  int = 1000
	current_group_id int = 1000
)

type PlainHandler struct {
	minioth *Minioth
}

/*  */
/* initialization routines. check if data directory is there, check if root user exists...*/
func (m *PlainHandler) Init() {
	dir := strings.Split(MINIOTH_GROUP, "/")
	if len(dir) == 0 {
		log.Fatalf("bad constant: %v", MINIOTH_GROUP)
	}
	err := os.MkdirAll(strings.Join(dir[:len(dir)-1], "/"), 0o644)
	if err != nil {
		log.Fatalf("couldn't instantiate the main files path: %v", err)
	}

	log.Print("[INIT]Checking if plain dir exists...")
	// Should check if root exists...
	if err := verifyFilePrefix(MINIOTH_PASSWD, m.minioth.Config.MINIOTH_ACCESS_KEY); err != nil {
		// Add root user
		m.insertAdminAndMainGroups(
			ut.User{
				Username: m.minioth.Config.MINIOTH_ACCESS_KEY,
				Password: ut.Password{
					Hashpass: m.minioth.Config.MINIOTH_SECRET_KEY,
				},
				Uid:    0,
				Pgroup: 0,
				Info:   "administrator",
				Home:   "/",
				Shell:  "/bin/gshell",
			},
			[]ut.Group{
				{
					Groupname: "admin",
					Gid:       0,
					Users: []ut.User{
						{
							Username: m.minioth.Config.MINIOTH_ACCESS_KEY,
						},
					},
				},
				{
					Groupname: "mod",
					Gid:       100,
					Users:     []ut.User{},
				},
				{
					Groupname: "user",
					Gid:       1000,
					Users:     []ut.User{},
				},
			},
		)
	}

	log.Print("[INIT]Syncing user ids")
	syncCurrentIds()

}

func (m *PlainHandler) insertAdminAndMainGroups(user ut.User, groups []ut.Group) error {
	// Open/Create files first to handle all file errors at once.
	file, err := os.OpenFile(MINIOTH_PASSWD, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("error opening file: %v", err)
		return err
	}
	defer file.Close()

	pfile, err := os.OpenFile(MINIOTH_SHADOW, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		log.Printf("error opening file: %v", err)
		return err
	}
	defer pfile.Close()

	gfile, err := os.OpenFile(MINIOTH_GROUP, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("error opening file: %v", err)
		return err
	}
	defer gfile.Close()

	// Generate password early, return early if failed...
	hashPass, err := hash([]byte(user.Password.Hashpass))
	if err != nil {
		log.Printf("failed to hash the pass... :%v", err)
		return err
	}

	uid := current_admin_id
	current_admin_id += 1

	fmt.Fprintf(file, ENTRY_MPASSWD_FORMAT, user.Username, PLACEHOLDER_PASS, uid, uid, user.Info, user.Home, user.Shell)
	fmt.Fprintf(pfile, ENTRY_MSHADOW_FORMAT, user.Username, hashPass, user.Password.LastPasswordChange, user.Password.MinimumPasswordAge, user.Password.MaximumPasswordAge, user.Password.WarningPeriod, user.Password.InactivityPeriod, user.Password.ExpirationDate, len(user.Password.Hashpass))

	for _, group := range groups {
		fmt.Fprintf(gfile, ENTRY_MGROUP_FORMAT, group.Groupname, group.Gid, ut.UsersToString(group.Users))
	}

	return nil
}

func (m *PlainHandler) Useradd(user ut.User) (int, int, error) {
	// Open/Create files first to handle all file errors at once.
	file, err := os.OpenFile(MINIOTH_PASSWD, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("error opening file: %v", err)
		return -1, -1, err
	}
	defer file.Close()

	pfile, err := os.OpenFile(MINIOTH_SHADOW, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		log.Printf("error opening file: %v", err)
		return -1, -1, err
	}
	defer pfile.Close()

	gfile, err := os.OpenFile(MINIOTH_GROUP, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("error opening file: %v", err)
		return -1, -1, err
	}
	defer gfile.Close()

	// Check if exists
	err = exists(user.Username, "users")
	if err != nil {
		log.Printf("error: user already exists: %v", err)
		return -1, -1, err
	}

	// Generate password early, return early if failed...
	hashPass, err := hash([]byte(user.Password.Hashpass))
	if err != nil {
		log.Printf("failed to hash the pass... :%v", err)
		return -1, -1, err
	}
	uid := current_user_id
	current_user_id += 1

	// obtain writelock
	writeLock.Lock()
	defer writeLock.Unlock()

	fmt.Fprintf(file, ENTRY_MPASSWD_FORMAT, user.Username, PLACEHOLDER_PASS, uid, uid, user.Info, user.Home, user.Shell)
	fmt.Fprintf(pfile, ENTRY_MSHADOW_FORMAT, user.Username, hashPass, user.Password.LastPasswordChange, user.Password.MinimumPasswordAge, user.Password.MaximumPasswordAge, user.Password.WarningPeriod, user.Password.InactivityPeriod, user.Password.ExpirationDate, len(user.Password.Hashpass))

	return uid, -1, nil
}

func (m *PlainHandler) Userdel(uid string) error {
	if uid == "" {
		log.Print("must provide a uid")
		return fmt.Errorf("must provide a uid")
	}

	f, err := os.OpenFile(MINIOTH_PASSWD, os.O_RDWR, 0o600)
	if err != nil {
		fmt.Printf("error opening file: %v", err)
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	var updated []string

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, DEL)
		if len(parts) < 3 {
			continue
		}
		if parts[2] != uid {
			updated = append(updated, line)
		}
	}

	writeLock.Lock()
	defer writeLock.Unlock()

	f, err = os.Create(MINIOTH_PASSWD)
	if err != nil {
		log.Print("failed to create the file")
		return err
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	for _, line := range updated {
		_, err := writer.WriteString(line + "\n")
		if err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}

	current_user_id -= 1

	return nil
}

func (m *PlainHandler) Usermod(user ut.User) error {
	f, err := os.OpenFile(MINIOTH_PASSWD, os.O_RDWR, 0o600)
	if err != nil {
		fmt.Printf("error opening file: %v", err)
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)

	var updated []string
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, DEL)
		if len(parts) < 3 {
			continue
		}

		if parts[2] != strconv.Itoa(user.Uid) {
			updated = append(updated, line)
		} else {
			parts[0] = user.Username
			parts[4] = user.Info
			parts[5] = user.Home
			parts[6] = user.Shell
			updated = append(updated, strings.Join(parts, DEL))
		}
	}

	writeLock.Lock()
	defer writeLock.Unlock()

	f, err = os.Create(MINIOTH_PASSWD)
	if err != nil {
		log.Print("failed to create the file")
		return err
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	for _, line := range updated {
		_, err := writer.WriteString(line + "\n")
		if err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}

	return nil
}

func (m *PlainHandler) Userpatch(uid string, fields map[string]any) error {
	f, err := os.OpenFile(MINIOTH_PASSWD, os.O_RDWR, 0o600)
	if err != nil {
		fmt.Printf("error opening file: %v", err)
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)

	var updated []string
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, DEL)
		if len(parts) < 3 {
			continue
		}

		if parts[2] != uid {
			updated = append(updated, line)
		} else {
			n, exists := fields["username"]
			if exists && n.(string) != "" {
				parts[0] = n.(string)
			}
			u, exists := fields["uid"]
			if exists && u.(string) != "" {
				parts[2] = u.(string)
			}
			p, exists := fields["pgroup"]
			if exists && p.(string) != "" {
				parts[3] = p.(string)
			}
			i, exists := fields["info"]
			if exists && i.(string) != "" {
				parts[4] = i.(string)
			}
			h, exists := fields["home"]
			if exists && h.(string) != "" {
				parts[5] = h.(string)
			}
			s, exists := fields["shell"]
			if exists && s.(string) != "" {
				parts[6] = s.(string)
			}
			updated = append(updated, strings.Join(parts, DEL))
		}
	}

	writeLock.Lock()
	defer writeLock.Unlock()

	f, err = os.Create(MINIOTH_PASSWD)
	if err != nil {
		log.Print("failed to create the file")
		return err
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	for _, line := range updated {
		_, err := writer.WriteString(line + "\n")
		if err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}
	return nil
}

func (m *PlainHandler) Groupadd(group ut.Group) (int, error) {
	file, err := os.OpenFile(MINIOTH_GROUP, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("error opening file: %v", err)
		return -1, err
	}
	defer file.Close()
	err = exists(group.Groupname, "groups")
	if err != nil {
		log.Printf("error: group already exists: %v", err)
		return -1, err
	}

	gid := current_group_id
	current_group_id += 1

	writeLock.Lock()
	defer writeLock.Unlock()
	fmt.Fprintf(file, ENTRY_MGROUP_FORMAT, group.Groupname, gid, group.Users)

	return -1, nil
}

func (m *PlainHandler) Groupdel(gid string) error {
	if gid == "" {
		log.Print("must provide a uid")
		return fmt.Errorf("must provide a uid")
	}

	f, err := os.OpenFile(MINIOTH_GROUP, os.O_RDWR, 0o600)
	if err != nil {
		fmt.Printf("error opening file: %v", err)
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)

	var updated []string
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, DEL)
		if len(parts) < 3 {
			continue
		}
		if parts[1] != gid {
			updated = append(updated, line)
		}
	}

	writeLock.Lock()
	defer writeLock.Unlock()

	f, err = os.Create(MINIOTH_GROUP)
	if err != nil {
		log.Print("failed to create the file")
		return err
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	for _, line := range updated {
		_, err := writer.WriteString(line + "\n")
		if err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}
	current_group_id -= 1
	return nil
}

func (m *PlainHandler) Groupmod(group ut.Group) error {
	f, err := os.OpenFile(MINIOTH_GROUP, os.O_RDWR, 0o600)
	if err != nil {
		fmt.Printf("error opening file: %v", err)
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)

	var updated []string
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, DEL)
		if len(parts) < 3 {
			continue
		}
		if parts[1] != strconv.Itoa(group.Gid) {
			updated = append(updated, line)
		} else {
			parts[0] = group.Groupname
			updated = append(updated, strings.Join(parts, DEL))
		}
	}

	writeLock.Lock()
	defer writeLock.Unlock()

	f, err = os.Create(MINIOTH_GROUP)
	if err != nil {
		log.Print("failed to create the file")
		return err
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	for _, line := range updated {
		_, err := writer.WriteString(line + "\n")
		if err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}
	return nil
}

func (m *PlainHandler) Grouppatch(gid string, fields map[string]any) error {
	f, err := os.OpenFile(MINIOTH_GROUP, os.O_RDWR, 0o600)
	if err != nil {
		fmt.Printf("error opening file: %v", err)
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)

	var updated []string
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, DEL)
		if len(parts) < 3 {
			continue
		}
		if parts[1] != gid {
			updated = append(updated, line)
		} else {
			newgr_name, exists := fields["groupname"]
			if exists && newgr_name.(string) != "" {
				parts[0] = newgr_name.(string)
			}

			newgid, exists := fields["gid"]
			if exists && newgid.(string) != "" {
				parts[1] = newgid.(string)
			}

			u, exist := fields["users"]
			if exist && u.(string) != "" {
				parts[2] = u.(string)
			}

			updated = append(updated, strings.Join(parts, DEL))
		}
	}

	writeLock.Lock()
	defer writeLock.Unlock()

	f, err = os.Create(MINIOTH_GROUP)
	if err != nil {
		log.Print("failed to create the file")
		return err
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	for _, line := range updated {
		_, err := writer.WriteString(line + "\n")
		if err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}
	return nil
}

func (m *PlainHandler) Select(id string) any {
	var param, value string
	parts := strings.Split(id, "?")
	if len(parts) > 1 {
		id = parts[0]
		parts_2 := strings.Split(parts[1], "=")
		if len(parts_2) > 1 {
			param = parts_2[0]
			value = parts_2[1]
		}
	}
	switch id {
	case "users":
		var users []ut.User
		f, err := os.Open(MINIOTH_PASSWD)
		if err != nil {
			log.Printf("error reading file: %v", err)
			return nil
		}
		defer f.Close()
		if param != "" && value != "" { // select a specified entry
			entry, err := getUserEntryById(value, f)
			if err != nil {
				log.Printf("failed to get entry")
				return nil
			}
			return entry
		} else {
			users, err = getUserEntries(f)
			if err != nil {
				log.Printf("failed to get group entries")
				return nil
			}
			return users
		}
	case "groups":
		var groups []ut.Group
		f, err := os.Open(MINIOTH_GROUP)
		if err != nil {
			log.Printf("error reading file: %v", err)
			return nil
		}
		defer f.Close()

		if param != "" && value != "" { // select a specified entry
			entry, err := getGroupEntryById(value, f)
			if err != nil {
				log.Printf("failed to get entry")
				return nil
			}
			return entry
		} else {
			groups, err = getGroupEntries(f)
			if err != nil {
				log.Printf("failed to get group entries")
				return nil
			}
			return groups
		}

	default:
		log.Print("Invalid id: " + id)
		return nil
	}
}

/* approval of minioth means, user exists and password is valid */
func (m *PlainHandler) Authenticate(username, password string) (ut.User, error) {
	var user ut.User

	pfile, err := os.Open(MINIOTH_SHADOW)
	if err != nil {
		log.Printf("failed to open file: %v", err)
		return user, err
	}
	defer pfile.Close()

	passline, pline, err := getEntry(username, pfile)
	if err != nil || pline == -1 {
		log.Printf("failed to search for pass: %v", err)
		return user, err
	}

	p := strings.SplitN(passline, DEL, 3)
	if len(p) != 3 {
		return user, fmt.Errorf("invalid shadow line retrieved")
	}
	hashpass := p[1]

	if verifyPass([]byte(hashpass), []byte(password)) {
		// fetch other userinformation.
		file, err := os.Open(MINIOTH_PASSWD)
		if err != nil {
			log.Printf("failed to open file: %v", err)
			return user, err
		}
		defer pfile.Close()

		userline, pline, err := getEntry(username, file)
		if err != nil || pline == -1 {
			log.Printf("failed to search for pass: %v", err)
			return user, err
		}
		p := strings.Split(userline, ":")
		if len(p) != 7 {
			return user, fmt.Errorf("invalid user entry")
		}
		user.Username = username
		user.Password = ut.Password{}
		user.Uid, err = strconv.Atoi(p[2])
		if err != nil {
			return user, fmt.Errorf("failed to atoi user id")
		}
		user.Pgroup, err = strconv.Atoi(p[3])
		if err != nil {
			return user, fmt.Errorf("failed to atoi user pgroup")
		}
		user.Info = p[4]
		user.Home = p[5]
		user.Shell = p[6]

		return user, nil
	} else {
		return user, fmt.Errorf("failed to authenticate, bad creds")
	}
}

func (m *PlainHandler) Passwd(username, password string) error {
	f, err := os.OpenFile(MINIOTH_SHADOW, os.O_RDWR, 0o600)
	if err != nil {
		fmt.Printf("error opening file: %v", err)
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)

	var updated []string
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, DEL)
		if len(parts) < 3 {
			continue
		}

		if parts[0] != username {
			updated = append(updated, line)
		} else {
			newPass, err := hash([]byte(password))
			if err != nil {
				log.Printf("Failed to hash the pass... :%v", err)
				return err
			}
			parts[1] = string(newPass)

			updated = append(updated, strings.Join(parts, DEL))
		}
	}

	writeLock.Lock()
	defer writeLock.Unlock()

	f, err = os.Create(MINIOTH_SHADOW)
	if err != nil {
		log.Print("failed to create the file")
		return err
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	for _, line := range updated {
		_, err := writer.WriteString(line + "\n")
		if err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}

	return nil
}

// unused for this handler
func (p *PlainHandler) Close() {
}

/* just read the first 256 bytes from a file...
* Used to check if root is entried.*/
func verifyFilePrefix(filePath, prefix string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	buffer := make([]byte, 256)

	_, err = file.Read(buffer)
	if err != nil {
		return fmt.Errorf("failed to read from file: %w", err)
	}

	if strings.Contains(string(buffer), prefix) {
		return nil
	}

	return fmt.Errorf("prefix doesn't match %s", prefix)
}

func exists(who, what string) error {
	if what == "users" {
		file, err := os.Open(MINIOTH_PASSWD)
		if err != nil {
			log.Printf("error opening file: %v", err)
			return err
		}
		defer file.Close()
		_, line, err := getEntry(who, file)
		if err == nil && line != -1 {
			return fmt.Errorf("user already exists")
		}
	} else if what == "groups" {
		file, err := os.Open(MINIOTH_GROUP)
		if err != nil {
			log.Printf("error opening file: %v", err)
			return err
		}
		defer file.Close()
		_, line, err := getEntry(who, file)
		if err == nil && line != -1 {
			return fmt.Errorf("group already exists")
		}
	} else {
		return fmt.Errorf("invalid search")
	}
	return nil
}

func getEntry(name string, file *os.File) (string, int, error) {
	if name == "" || file == nil {
		return "", -1, fmt.Errorf("must provide parameter")
	}
	scanner := bufio.NewScanner(file)

	lineIndex := 0
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, DEL, 2)
		if len(parts) != 2 {
			return "", -1, fmt.Errorf("no content found")
		}
		if name == parts[0] {
			return line, lineIndex, nil
		}

		lineIndex++

	}

	return "", -1, fmt.Errorf("not found")
}

func getUserEntries(f *os.File) ([]ut.User, error) {
	if f == nil {
		return nil, fmt.Errorf("must provide a file pointer")
	}
	// store shadow contents in mem
	sf, err := os.Open(MINIOTH_SHADOW)
	if err != nil {
		return nil, fmt.Errorf("failed to open shadow file")
	}
	shadowMap := map[string]ut.Password{}

	scanner := bufio.NewScanner(sf)
	for scanner.Scan() {
		line := scanner.Text()
		p := strings.SplitN(line, DEL, 8)
		if len(p) != 8 {
			return nil, fmt.Errorf("invalid shadow entries format")
		}
		pass := ut.Password{
			Hashpass:           p[1],
			LastPasswordChange: p[2],
			MinimumPasswordAge: p[3],
			MaximumPasswordAge: p[4],
			WarningPeriod:      p[5],
			InactivityPeriod:   p[6],
			ExpirationDate:     p[7],
		}
		shadowMap[p[0]] = pass
	}
	sf.Close()

	var users []ut.User
	scanner = bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		p := strings.SplitN(line, DEL, 7)
		if len(p) != 7 {
			return nil, fmt.Errorf("invalid passwd entries format")
		}
		uid, err := strconv.Atoi(p[2])
		if err != nil {
			return nil, fmt.Errorf("failed to atoi  uid, invalid passwd entry")
		}
		pgroup, err := strconv.Atoi(p[3])
		if err != nil {
			return nil, fmt.Errorf("failed to atoi pgroup, invalid passwd entry")
		}
		user := ut.User{
			Username: p[0],
			Password: shadowMap[p[0]],
			Uid:      uid,
			Pgroup:   pgroup,
			Info:     p[4],
			Home:     p[5],
			Shell:    p[6],
		}

		users = append(users, user)

	}
	return users, nil
}

func getUserEntryById(id string, f *os.File) (ut.User, error) {
	if f == nil {
		return ut.User{}, fmt.Errorf("must provide a file pointer")
	}
	scanner := bufio.NewScanner(f)

	sf, err := os.Open(MINIOTH_SHADOW)
	if err != nil {
		log.Printf("failed to open shadow file")
		return ut.User{}, fmt.Errorf("failed to open shadow file: %v", err)
	}
	defer sf.Close()

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, DEL, 7)
		if len(parts) != 7 {
			return ut.User{}, fmt.Errorf("no content found")
		}
		if id == parts[2] {
			pass_line, _, err := getEntry(parts[0], sf)
			if err != nil {
				log.Printf("failed to retrieve password entry: %v", err)
				return ut.User{}, fmt.Errorf("failed to retrieve password entry")
			}
			// parse password
			pp := strings.SplitN(pass_line, DEL, 8)
			if len(pp) != 8 {
				return ut.User{}, fmt.Errorf("invalid shadow entry")
			}
			pass := ut.Password{
				Hashpass:           pp[1],
				LastPasswordChange: pp[2],
				MinimumPasswordAge: pp[3],
				MaximumPasswordAge: pp[4],
				WarningPeriod:      pp[5],
				InactivityPeriod:   pp[6],
				ExpirationDate:     pp[7],
			}

			uid, err := strconv.Atoi(parts[2])
			if err != nil {
				return ut.User{}, fmt.Errorf("failed to atoi  uid, invalid passwd entry")
			}
			pgroup, err := strconv.Atoi(parts[3])
			if err != nil {
				return ut.User{}, fmt.Errorf("failed to atoi pgroup, invalid passwd entry")
			}
			u := ut.User{
				Username: parts[0],
				Password: pass,
				Uid:      uid,
				Pgroup:   pgroup,
				Info:     parts[4],
				Home:     parts[5],
				Shell:    parts[6],
			}

			return u, nil
		}
	}
	return ut.User{}, fmt.Errorf("entry not found")
}

func getGroupEntries(f *os.File) ([]ut.Group, error) {
	if f == nil {
		return nil, fmt.Errorf("must provide a file pointer")
	}
	var groups []ut.Group
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, DEL, 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid entry format")
		}
		var group ut.Group
		group.Groupname = parts[0]
		gid, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("failed to atoi gid, invalid entry format")
		}
		group.Gid = gid
		users := strings.Split(strings.TrimSpace(parts[2]), ",")
		var u []ut.User
		for _, user := range users {
			u = append(u, ut.User{Username: user})
		}
		group.Users = u
		groups = append(groups, group)
	}
	return groups, nil
}

func getGroupEntryById(gid string, file *os.File) (ut.Group, error) {
	if gid == "" || file == nil {
		return ut.Group{}, fmt.Errorf("must provide parameter")
	}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, DEL, 3)
		if len(parts) != 3 {
			return ut.Group{}, fmt.Errorf("no content found")
		}
		if gid == parts[1] {
			gid, err := strconv.Atoi(parts[1])
			if err != nil {
				return ut.Group{}, fmt.Errorf("failed to atoi gid, bad entry format")
			}
			var u []ut.User
			users := strings.Split(strings.TrimSpace(parts[2]), ",")
			for _, user := range users {
				u = append(u, ut.User{Username: user})
			}
			g := ut.Group{
				Groupname: parts[0],
				Gid:       gid,
				Users:     u,
			}
			return g, nil
		}

	}

	return ut.Group{}, fmt.Errorf("not found")
}

func syncCurrentIds() {
	f, err := os.Open(MINIOTH_PASSWD)
	if err != nil {
		panic("couldn't open passwd")
	}
	defer f.Close()

	currentUids := []string{}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, DEL)
		if len(parts) != 7 {
			continue
		}
		id := parts[2]
		currentUids = append(currentUids, id)
	}

	for _, str_id := range currentUids {
		iuid, err := strconv.Atoi(str_id)
		if err != nil {
			log.Fatalf("failed to parse id: %v", err)
		}

		if iuid < 100 {
			current_admin_id = max(iuid, current_admin_id) + 1
		} else if iuid < 1000 {
			current_mod_id = max(iuid, current_mod_id) + 1
		} else {
			current_user_id = max(iuid, current_user_id) + 1
		}
	}

	f, err = os.Open(MINIOTH_GROUP)
	if err != nil {
		panic("couldn't open group")
	}
	defer f.Close()

	currentGids := []string{}

	scanner = bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, DEL)
		if len(parts) != 3 {
			continue
		}
		id := parts[1]
		currentGids = append(currentGids, id)
	}

	for _, str_id := range currentGids {
		igid, err := strconv.Atoi(str_id)
		if err != nil {
			log.Fatalf("failed to parse id: %v", err)
		}
		current_group_id = max(igid, current_mod_id) + 1
	}

}
