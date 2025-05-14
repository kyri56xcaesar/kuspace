package fslite

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	MINIMUM_USER_LENGTH = 3
	MINIMUM_PASS_LENGTH = 5
)

var (
	HASH_COST        int     = 4
	jwtValidityHours float64 = 4
	jwtSecretKey     string  = "r4nd0m"
	usernameRegex            = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	passwordRegex            = regexp.MustCompile(`^[a-zA-Z0-9!@#\$%\^&\*]+$`)
)

// Admin represents an admin login or registration object.
// @Description Admin login/registration payload
type Admin struct {
	ID       uuid.UUID `db:"id" json:"id,omitempty"`
	Username string    `db:"username" json:"username"`
	Password string    `db:"password" json:"password"`
}

func (a *Admin) ptrFields() []any {
	return []any{&a.ID, &a.Username, &a.Password}
}

func (adm *Admin) validate() error {
	if len(adm.Username) < MINIMUM_USER_LENGTH {
		return fmt.Errorf("username length too small")
	}

	if len(adm.Password) < MINIMUM_PASS_LENGTH {
		return fmt.Errorf("password length too small")
	}

	if !usernameRegex.MatchString(adm.Username) {
		return errors.New("username contains invalid characters")
	}

	if !passwordRegex.MatchString(adm.Password) {
		return errors.New("password contains invalid characters")
	}

	return nil
}

func (fsl *FsLite) insertAdmin(username, password string) (admin Admin, err error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("failed to retrieve db conn: %v", err)
		return Admin{}, err
	}

	id := uuid.New()
	admin = Admin{
		ID:       id,
		Username: username,
		Password: password,
	}
	err = admin.validate()
	if err != nil {
		log.Printf("failed to validate the user: %v", err)
		return Admin{}, err
	}

	query := `
	INSERT INTO 
		user_admin (uuid, username, hashpass)
	VALUES
		(?, ?, ?);
	`

	hashpass, err := hash([]byte(password))
	if err != nil {
		log.Printf("failed to hash the password: %v", err)
		return Admin{}, err
	}
	admin.Password = string(hashpass)

	log.Printf("inserting default user: %+v", admin)

	_, err = db.Exec(query, id, username, hashpass)
	if err != nil {
		log.Printf("failed to execute query: %v", err)
	}

	return admin, err
}

func (fsl *FsLite) authenticateAdmin(username, password string) (token string, err error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("failed to retrieve db conn: %v", err)
		return "", err
	}
	query := `SELECT * FROM user_admin WHERE username = ?`

	admin := Admin{}
	err = db.QueryRow(query, username).Scan(admin.ptrFields()...)
	if err != nil {
		log.Printf("failed to query and scan correctly: %v", err)
		return "", err
	}

	err = verifyPass(admin.Password, password)
	if err != nil {
		log.Printf("password didn't match: %v", err)
		return "", err
	}

	token, err = generateAccessJWT(admin.ID.String(), admin.Username)
	if err != nil {
		log.Printf("failed generating jwt token: %v", err)
		return "", err
	}
	return token, err
}

func hash(password []byte) ([]byte, error) {
	return bcrypt.GenerateFromPassword(password, HASH_COST)
}

/* check if a passowrd is correct */
func verifyPass(hashedPass, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hashedPass), []byte(password)); err != nil {
		return fmt.Errorf("fail")
	}
	return nil
}

// jwt
type CustomClaims struct {
	ID       string `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func generateAccessJWT(userID, username string) (string, error) {
	// Set the claims for the token
	claims := CustomClaims{
		ID:       userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "fslite",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * time.Duration(jwtValidityHours))),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID,
		},
	}

	// Create the token using the HS256 signing method
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token using the secret key
	tokenString, err := token.SignedString([]byte(jwtSecretKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

func decodeJWT(tokenString string) (bool, *CustomClaims, error) {
	// Parse and validate the token
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecretKey), nil
	})

	if err != nil {
		log.Printf("%v token, exiting", token)
		return false, nil, err
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok {
		log.Printf("not okay when retrieving claims")
		return false, nil, errors.New("invalid claims")
	}

	return true, claims, nil
}
