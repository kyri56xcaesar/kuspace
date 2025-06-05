package minioth

import (
	"bufio"
	"fmt"
	ut "kyri56xcaesar/kuspace/internal/utils"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
)

const (
	miniothPasswd       = "data/plain/minioth/mpasswd"
	miniothGroup        = "data/plain/minioth/mgroup"
	miniothShadow       = "data/plain/minioth/mshadow"
	placeholderPass     = "bcrypted_password_hash" // what is shown in passwd for password
	del                 = ":"                      //delimitter
	entryMPasswdFormat  = "%s" + del + "%s" + del + "%v" + del + "%v" + del + "%s" + del + "%s" + del + "%s\n"
	entryMPshadowFormat = "%s" + del + "%s" + del + "%s" + del + "%s" + del + "%s" + del + "%s" + del + "%s" + del + "%s" + del + "%v\n"
	entryMGroupFormat   = "%s" + del + "%v" + del + "%v\n"
	// username:password ref:uuid:guid:info:home:shell
	// username:hashed_pass:last_password_change:min_password_age:max_password_age:warning_period:inactivity_period:expiration_date: (bunch of times)
	// groupname:group_id:users
)

var (
	writeLock      = sync.Mutex{}
	currentAdminID = 0
	currentModID   = 100
	currentUserID  = 1000
	currentGroupID = 1000
)

// PlainHandler struct holding of the Plain Minioth Handler,
// holds reference to centralle..
type PlainHandler struct {
	minioth *Minioth
}

// Init method has initialization routines. check if data directory is there, check if root user exists...*/
func (m *PlainHandler) Init() {
	dir := strings.Split(miniothGroup, "/")
	if len(dir) == 0 {
		log.Fatalf("bad constant: %v", miniothGroup)
	}
	err := os.MkdirAll(strings.Join(dir[:len(dir)-1], "/"), 0o644)
	if err != nil {
		log.Fatalf("couldn't instantiate the main files path: %v", err)
	}

	log.Print("[INIT]Checking if plain dir exists...")
	// Should check if root exists...
	if err := verifyFilePrefix(miniothPasswd, m.minioth.Config.MinioAccessKey); err != nil {
		// Add root user
		err = m.insertAdminAndMainGroups(
			ut.User{
				Username: m.minioth.Config.MinioAccessKey,
				Password: ut.Password{
					Hashpass: m.minioth.Config.MiniothSecretKey,
				},
				UID:    0,
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
							Username: m.minioth.Config.MinioAccessKey,
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
		if err != nil {
			log.Printf("failed to insert main user and groups: %v", err)
		}
	}

	log.Print("[INIT]Syncing user ids")
	syncCurrentIDs()

}

func (m *PlainHandler) insertAdminAndMainGroups(user ut.User, groups []ut.Group) error {
	// Open/Create files first to handle all file errors at once.
	file, err := os.OpenFile(miniothPasswd, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("error opening file: %v", err)
		return err
	}
	defer func() {
		err := file.Close()
		if err != nil {
			log.Printf("failed to close the file: %v", err)
		}
	}()

	pfile, err := os.OpenFile(miniothShadow, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		log.Printf("error opening file: %v", err)
		return err
	}
	defer func() {
		err := pfile.Close()
		if err != nil {
			log.Printf("failed to close the file: %v", err)
		}
	}()

	gfile, err := os.OpenFile(miniothGroup, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("error opening file: %v", err)
		return err
	}
	defer func() {
		err := gfile.Close()
		if err != nil {
			log.Printf("failed to close the file: %v", err)
		}
	}()

	// Generate password early, return early if failed...
	hashPass, err := hash([]byte(user.Password.Hashpass))
	if err != nil {
		log.Printf("failed to hash the pass... :%v", err)
		return err
	}

	uid := currentAdminID
	currentAdminID++

	_, err = fmt.Fprintf(file, entryMPasswdFormat, user.Username, placeholderPass, uid, uid, user.Info, user.Home, user.Shell)
	if err != nil {
		log.Printf("failed to write entry: %v", err)
	}
	_, err = fmt.Fprintf(pfile, entryMPshadowFormat, user.Username, hashPass, user.Password.LastPasswordChange, user.Password.MinimumPasswordAge, user.Password.MaximumPasswordAge, user.Password.WarningPeriod, user.Password.InactivityPeriod, user.Password.ExpirationDate, len(user.Password.Hashpass))
	if err != nil {
		log.Printf("failed to write entry: %v", err)
	}
	for _, group := range groups {
		_, err = fmt.Fprintf(gfile, entryMGroupFormat, group.Groupname, group.Gid, ut.UsersToString(group.Users))
		if err != nil {
			log.Printf("failed to write entry: %v", err)
		}
	}

	return nil
}

// Useradd method of the Plain Minioth Handler
func (m *PlainHandler) Useradd(user ut.User) (int, int, error) {
	// Open/Create files first to handle all file errors at once.
	file, err := os.OpenFile(miniothPasswd, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("error opening file: %v", err)
		return -1, -1, err
	}
	defer func() {
		err := file.Close()
		if err != nil {
			log.Printf("failed to close the file: %v", err)
		}
	}()

	pfile, err := os.OpenFile(miniothShadow, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		log.Printf("error opening file: %v", err)
		return -1, -1, err
	}
	defer func() {
		err := pfile.Close()
		if err != nil {
			log.Printf("failed to close the file: %v", err)
		}
	}()

	gfile, err := os.OpenFile(miniothGroup, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("error opening file: %v", err)
		return -1, -1, err
	}
	defer func() {
		err := gfile.Close()
		if err != nil {
			log.Printf("failed to close the file: %v", err)
		}
	}()

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
	uid := currentUserID
	currentUserID++

	// obtain writelock
	writeLock.Lock()
	defer writeLock.Unlock()

	_, err = fmt.Fprintf(file, entryMPasswdFormat, user.Username, placeholderPass, uid, uid, user.Info, user.Home, user.Shell)
	if err != nil {
		log.Printf("failed to write to file: %v", err)
		return -1, -1, err
	}
	_, err = fmt.Fprintf(pfile, entryMPshadowFormat, user.Username, hashPass, user.Password.LastPasswordChange, user.Password.MinimumPasswordAge, user.Password.MaximumPasswordAge, user.Password.WarningPeriod, user.Password.InactivityPeriod, user.Password.ExpirationDate, len(user.Password.Hashpass))
	if err != nil {
		log.Printf("failed to write to file: %v", err)
		return -1, -1, err
	}

	return uid, -1, nil
}

// Userdel method of the Plain Minioth Handler
func (m *PlainHandler) Userdel(uid string) error {
	if uid == "" {
		log.Print("must provide a uid")
		return fmt.Errorf("must provide a uid")
	}

	f, err := os.OpenFile(miniothPasswd, os.O_RDWR, 0o600)
	if err != nil {
		fmt.Printf("error opening file: %v", err)
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Printf("failed to close the file: %v", err)
		}
	}()

	scanner := bufio.NewScanner(f)

	var updated []string

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, del)
		if len(parts) < 3 {
			continue
		}
		if parts[2] != uid {
			updated = append(updated, line)
		}
	}

	writeLock.Lock()
	defer writeLock.Unlock()

	f, err = os.Create(miniothPasswd)
	if err != nil {
		log.Print("failed to create the file")
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Printf("failed to close the file: %v", err)
		}
	}()

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

	currentUserID--

	return nil
}

// Usermod method of the Plain Minioth Handler
func (m *PlainHandler) Usermod(user ut.User) error {
	f, err := os.OpenFile(miniothPasswd, os.O_RDWR, 0o600)
	if err != nil {
		fmt.Printf("error opening file: %v", err)
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Printf("failed to close the file: %v", err)
		}
	}()
	scanner := bufio.NewScanner(f)

	var updated []string
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, del)
		if len(parts) < 3 {
			continue
		}

		if parts[2] != strconv.Itoa(user.UID) {
			updated = append(updated, line)
		} else {
			parts[0] = user.Username
			parts[4] = user.Info
			parts[5] = user.Home
			parts[6] = user.Shell
			updated = append(updated, strings.Join(parts, del))
		}
	}

	writeLock.Lock()
	defer writeLock.Unlock()

	f, err = os.Create(miniothPasswd)
	if err != nil {
		log.Print("failed to create the file")
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Printf("failed to close the file: %v", err)
		}
	}()

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

// Userpatch method of the Plain Minioth Handler
func (m *PlainHandler) Userpatch(uid string, fields map[string]any) error {
	f, err := os.OpenFile(miniothPasswd, os.O_RDWR, 0o600)
	if err != nil {
		fmt.Printf("error opening file: %v", err)
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Printf("failed to close the file: %v", err)
		}
	}()
	scanner := bufio.NewScanner(f)

	var updated []string
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, del)
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
			updated = append(updated, strings.Join(parts, del))
		}
	}

	writeLock.Lock()
	defer writeLock.Unlock()

	f, err = os.Create(miniothPasswd)
	if err != nil {
		log.Print("failed to create the file")
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Printf("failed to close the file: %v", err)
		}
	}()

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

// Groupadd method of the Plain Minioth Handler
func (m *PlainHandler) Groupadd(group ut.Group) (int, error) {
	file, err := os.OpenFile(miniothGroup, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("error opening file: %v", err)
		return -1, err
	}
	defer func() {
		err := file.Close()
		if err != nil {
			log.Printf("failed to close the file: %v", err)
		}
	}()

	err = exists(group.Groupname, "groups")
	if err != nil {
		log.Printf("error: group already exists: %v", err)
		return -1, err
	}

	gid := currentGroupID
	currentGroupID++

	writeLock.Lock()
	defer writeLock.Unlock()
	_, err = fmt.Fprintf(file, entryMGroupFormat, group.Groupname, gid, group.Users)
	if err != nil {
		log.Printf("failed to write to file: %v", err)
	}

	return -1, err
}

// Groupdel method of the Plain Minioth Handler
func (m *PlainHandler) Groupdel(gid string) error {
	if gid == "" {
		log.Print("must provide a uid")
		return fmt.Errorf("must provide a uid")
	}

	f, err := os.OpenFile(miniothGroup, os.O_RDWR, 0o600)
	if err != nil {
		fmt.Printf("error opening file: %v", err)
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Printf("failed to close the file: %v", err)
		}
	}()
	scanner := bufio.NewScanner(f)

	var updated []string
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, del)
		if len(parts) < 3 {
			continue
		}
		if parts[1] != gid {
			updated = append(updated, line)
		}
	}

	writeLock.Lock()
	defer writeLock.Unlock()

	f, err = os.Create(miniothGroup)
	if err != nil {
		log.Print("failed to create the file")
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Printf("failed to close the file: %v", err)
		}
	}()

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
	currentGroupID--
	return nil
}

// Groupmod method of the Plain Minioth Handler
func (m *PlainHandler) Groupmod(group ut.Group) error {
	f, err := os.OpenFile(miniothGroup, os.O_RDWR, 0o600)
	if err != nil {
		fmt.Printf("error opening file: %v", err)
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Printf("failed to close the file: %v", err)
		}
	}()
	scanner := bufio.NewScanner(f)

	var updated []string
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, del)
		if len(parts) < 3 {
			continue
		}
		if parts[1] != strconv.Itoa(group.Gid) {
			updated = append(updated, line)
		} else {
			parts[0] = group.Groupname
			updated = append(updated, strings.Join(parts, del))
		}
	}

	writeLock.Lock()
	defer writeLock.Unlock()

	f, err = os.Create(miniothGroup)
	if err != nil {
		log.Print("failed to create the file")
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Printf("failed to close the file: %v", err)
		}
	}()

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

// Grouppatch method of the Plain Minioth Hadler
func (m *PlainHandler) Grouppatch(gid string, fields map[string]any) error {
	f, err := os.OpenFile(miniothGroup, os.O_RDWR, 0o600)
	if err != nil {
		fmt.Printf("error opening file: %v", err)
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Printf("failed to close the file: %v", err)
		}
	}()
	scanner := bufio.NewScanner(f)

	var updated []string
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, del)
		if len(parts) < 3 {
			continue
		}
		if parts[1] != gid {
			updated = append(updated, line)
		} else {
			newgrName, exists := fields["groupname"]
			if exists && newgrName.(string) != "" {
				parts[0] = newgrName.(string)
			}

			newgid, exists := fields["gid"]
			if exists && newgid.(string) != "" {
				parts[1] = newgid.(string)
			}

			u, exist := fields["users"]
			if exist && u.(string) != "" {
				parts[2] = u.(string)
			}

			updated = append(updated, strings.Join(parts, del))
		}
	}

	writeLock.Lock()
	defer writeLock.Unlock()

	f, err = os.Create(miniothGroup)
	if err != nil {
		log.Print("failed to create the file")
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Printf("failed to close the file: %v", err)
		}
	}()

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

// Select method of the Plain Minioth Handler
func (m *PlainHandler) Select(id string) any {
	var param, value string
	parts := strings.Split(id, "?")
	if len(parts) > 1 {
		id = parts[0]
		parts2 := strings.Split(parts[1], "=")
		if len(parts2) > 1 {
			param = parts2[0]
			value = parts2[1]
		}
	}
	switch id {
	case "users":
		var users []ut.User
		f, err := os.Open(miniothPasswd)
		if err != nil {
			log.Printf("error reading file: %v", err)
			return nil
		}
		defer func() {
			err := f.Close()
			if err != nil {
				log.Printf("failed to close the file: %v", err)
			}
		}()
		if param != "" && value != "" { // select a specified entry
			entry, err := getUserEntryByID(value, f)
			if err != nil {
				log.Printf("failed to get entry")
				return nil
			}
			g, err := os.Open(miniothGroup)
			if err != nil {
				log.Printf("error reading file: %v", err)
				return nil
			}
			defer func() {
				err := g.Close()
				if err != nil {
					log.Printf("failed to close the file: %v", err)
				}
			}()
			groups, err := getUserGroups(entry.Username, g)
			if err != nil {
				log.Printf("error getting user groups")
				return nil
			}
			entry.Groups = groups

			return entry
		}
		users, err = getUserEntries(f)
		if err != nil {
			log.Printf("failed to get group entries")
			return nil
		}
		return users

	case "groups":
		var groups []ut.Group
		f, err := os.Open(miniothGroup)
		if err != nil {
			log.Printf("error reading file: %v", err)
			return nil
		}
		defer func() {
			err := f.Close()
			if err != nil {
				log.Printf("failed to close the file: %v", err)
			}
		}()

		if param != "" && value != "" { // select a specified entry
			entry, err := getGroupEntryByID(value, f)
			if err != nil {
				log.Printf("failed to get entry")
				return nil
			}
			return entry
		}
		groups, err = getGroupEntries(f)
		if err != nil {
			log.Printf("failed to get group entries")
			return nil
		}
		return groups

	default:
		log.Print("Invalid id: " + id)
		return nil
	}
}

// Authenticate method of the Plain Minioth Handler
/* approval of minioth means, user exists and password is valid */
func (m *PlainHandler) Authenticate(username, password string) (ut.User, error) {
	var user ut.User

	pfile, err := os.Open(miniothShadow)
	if err != nil {
		log.Printf("failed to open file: %v", err)
		return user, err
	}
	defer func() {
		err := pfile.Close()
		if err != nil {
			log.Printf("failed to close the file: %v", err)
		}
	}()

	passline, pline, err := getEntry(username, pfile)
	if err != nil || pline == -1 {
		log.Printf("failed to search for pass: %v", err)
		return user, err
	}

	p := strings.SplitN(passline, del, 3)
	if len(p) != 3 {
		return user, fmt.Errorf("invalid shadow line retrieved")
	}
	hashpass := p[1]

	if verifyPass([]byte(hashpass), []byte(password)) {
		// fetch other userinformation.
		file, err := os.Open(miniothPasswd)
		if err != nil {
			log.Printf("failed to open file: %v", err)
			return user, err
		}
		defer func() {
			err := pfile.Close()
			if err != nil {
				log.Printf("failed to close the file: %v", err)
			}
		}()

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
		user.UID, err = strconv.Atoi(p[2])
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

		// we need to fetch his groups as well
		gfile, err := os.Open(miniothGroup)
		if err != nil {
			log.Printf("failed to open file: %v", err)
			return user, err
		}
		defer func() {
			err := gfile.Close()
			if err != nil {
				log.Printf("failed to close the file: %v", err)
			}
		}()
		groups, err := getUserGroups(username, gfile)
		if err != nil {
			log.Printf("failed to get the user groups: %v", err)
			return user, err
		}
		user.Groups = groups

		return user, nil
	}
	return user, fmt.Errorf("failed to authenticate, bad creds")
}

// Passwd method of the Plain Minioth Handler
func (m *PlainHandler) Passwd(username, password string) error {
	f, err := os.OpenFile(miniothShadow, os.O_RDWR, 0o600)
	if err != nil {
		log.Printf("error opening file: %v", err)
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Printf("failed to close the file: %v", err)
		}
	}()
	scanner := bufio.NewScanner(f)

	var updated []string
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, del)
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

			updated = append(updated, strings.Join(parts, del))
		}
	}

	writeLock.Lock()
	defer writeLock.Unlock()

	f, err = os.Create(miniothShadow)
	if err != nil {
		log.Print("failed to create the file")
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Printf("failed to close file: %v", err)
		}
	}()

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

// Purge method of the Plain Minioth Handler
/* NOTE: irrelevant atm
* delete the 3 state files */
// Purge (equivalent to Destrutor) is supposed to destroy the object and its deps.
func (m *PlainHandler) Purge() {
	log.Print("Purging everything...")

	_, err := os.Stat("data/plain")
	if err == nil {
		log.Print("data/plain dir exist")

		err = os.Remove(miniothPasswd)
		if err != nil {
			log.Print(err)
		}
		err = os.Remove(miniothGroup)
		if err != nil {
			log.Print(err)
		}
		err = os.Remove(miniothShadow)
		if err != nil {
			log.Print(err)
		}
		err = os.Remove("data/plain")
		if err != nil {
			log.Print(err)
		}
	}

	_, err = os.Stat("data/db")
	if err == nil {
		log.Print("data/db dir exists")
		err = os.Remove("data/*.db")
		if err != nil {
			log.Print(err)
		}

		err = os.Remove("data/db")
		if err != nil {
			log.Print(err)
		}
	}
}

// Close method of the Plain Minioth Handler
// unused for this handler
func (m *PlainHandler) Close() {
}

/* just read the first 256 bytes from a file...
* Used to check if root is entried.*/
func verifyFilePrefix(filePath, prefix string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		err := file.Close()
		if err != nil {
			log.Printf("failed to close file: %v", err)
		}
	}()

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
		file, err := os.Open(miniothPasswd)
		if err != nil {
			log.Printf("error opening file: %v", err)
			return err
		}
		defer func() {
			err := file.Close()
			if err != nil {
				log.Printf("failed to close file: %v", err)
			}
		}()
		_, line, err := getEntry(who, file)
		if err == nil && line != -1 {
			return fmt.Errorf("user already exists")
		}
	} else if what == "groups" {
		file, err := os.Open(miniothGroup)
		if err != nil {
			log.Printf("error opening file: %v", err)
			return err
		}
		defer func() {
			err := file.Close()
			if err != nil {
				log.Printf("failed to close file: %v", err)
			}
		}()
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
		parts := strings.SplitN(line, del, 2)
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
	sf, err := os.Open(miniothShadow)
	if err != nil {
		return nil, fmt.Errorf("failed to open shadow file")
	}
	shadowMap := map[string]ut.Password{}

	scanner := bufio.NewScanner(sf)
	for scanner.Scan() {
		line := scanner.Text()
		p := strings.SplitN(line, del, 8)
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
	err = sf.Close()
	if err != nil {
		log.Printf("failed to close the shadow file: %v", err)
		return nil, err
	}

	var users []ut.User
	scanner = bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		p := strings.SplitN(line, del, 7)
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
			UID:      uid,
			Pgroup:   pgroup,
			Info:     p[4],
			Home:     p[5],
			Shell:    p[6],
		}

		users = append(users, user)

	}
	return users, nil
}

func getUserEntryByID(id string, f *os.File) (ut.User, error) {
	if f == nil {
		return ut.User{}, fmt.Errorf("must provide a file pointer")
	}
	scanner := bufio.NewScanner(f)

	sf, err := os.Open(miniothShadow)
	if err != nil {
		log.Printf("failed to open shadow file")
		return ut.User{}, fmt.Errorf("failed to open shadow file: %v", err)
	}
	defer func() {
		err := sf.Close()
		if err != nil {
			log.Printf("failed to close the file: %v", err)
		}
	}()

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, del, 7)
		if len(parts) != 7 {
			return ut.User{}, fmt.Errorf("no content found")
		}
		if id == parts[2] {
			passLine, _, err := getEntry(parts[0], sf)
			if err != nil {
				log.Printf("failed to retrieve password entry: %v", err)
				return ut.User{}, fmt.Errorf("failed to retrieve password entry")
			}
			// parse password
			pp := strings.SplitN(passLine, del, 8)
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
				UID:      uid,
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
		parts := strings.SplitN(line, del, 3)
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

func getGroupEntryByID(gid string, file *os.File) (ut.Group, error) {
	if gid == "" || file == nil {
		return ut.Group{}, fmt.Errorf("must provide parameter")
	}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, del, 3)
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

func getUserGroups(username string, file *os.File) ([]ut.Group, error) {
	if username == "" || file == nil {
		return nil, fmt.Errorf("must provide parameters")
	}
	scanner := bufio.NewScanner(file)

	var groups []ut.Group
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, del, 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid group format entry")
		}
		if strings.Contains(parts[2], username) { // we have a group where the user belongs
			gid, err := strconv.Atoi(parts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid group format entry")
			}
			group := ut.Group{
				Groupname: parts[0],
				Gid:       gid,
			}
			groups = append(groups, group)
		}

	}
	return groups, nil
}

func syncCurrentIDs() {
	f, err := os.Open(miniothPasswd)
	if err != nil {
		panic("couldn't open passwd")
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Printf("failed to close file: %v", err)
		}
	}()

	currentUids := []string{}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, del)
		if len(parts) != 7 {
			continue
		}
		id := parts[2]
		currentUids = append(currentUids, id)
	}

	for _, strID := range currentUids {
		iuid, err := strconv.Atoi(strID)
		if err != nil {
			log.Fatalf("failed to parse id: %v", err)
		}

		if iuid < 100 {
			currentAdminID = max(iuid, currentAdminID) + 1
		} else if iuid < 1000 {
			currentModID = max(iuid, currentModID) + 1
		} else {
			currentUserID = max(iuid, currentUserID) + 1
		}
	}

	f, err = os.Open(miniothGroup)
	if err != nil {
		panic("couldn't open group")
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Printf("failed to close file: %v", err)
		}
	}()

	currentGids := []string{}

	scanner = bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, del)
		if len(parts) != 3 {
			continue
		}
		id := parts[1]
		currentGids = append(currentGids, id)
	}

	for _, strID := range currentGids {
		igid, err := strconv.Atoi(strID)
		if err != nil {
			log.Fatalf("failed to parse id: %v", err)
		}
		currentGroupID = max(igid, currentModID) + 1
	}

}
