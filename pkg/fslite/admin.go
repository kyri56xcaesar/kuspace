// Package fslite provides functionality for authentication. Only admin user management,
// registration, authentication, password hashing, and JWT tokens issuing
package fslite

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
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
	minimumUserLength = 3
	minimumPassLength = 8
	maximumPassLength = 72 // bcrypt ignores anything longer
)

var (
	// usernameRegex is the regular expression for validating usernames.
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	// passwordRegex allows any printable ASCII except spaces.
	passwordRegex = regexp.MustCompile(`^[\x21-\x7e]+$`)
)

// Admin represents an admin login or registration object.
// @Description Admin login/registration payload
type Admin struct {
	ID       uuid.UUID `db:"id"       json:"id,omitempty"`
	Username string    `db:"username" json:"username"`
	Password string    `db:"password" json:"password"`
}

// ptrFields returns pointers to the fields of the Admin struct.
// Useful for scanning database rows into the struct.
func (a *Admin) ptrFields() []any {
	return []any{&a.ID, &a.Username, &a.Password}
}

// validate checks the Admin struct fields for validity, including length and allowed characters.
func (a *Admin) validate() error {
	if len(a.Username) < minimumUserLength {
		return errors.New("username length too small")
	}

	if len(a.Password) < minimumPassLength {
		return errors.New("password length too small")
	}
	if len(a.Password) > maximumPassLength {
		return errors.New("password too long")
	}

	if !usernameRegex.MatchString(a.Username) {
		return errors.New("username contains invalid characters")
	}

	if !passwordRegex.MatchString(a.Password) {
		return errors.New("password contains invalid characters")
	}

	return nil
}

// insertAdmin creates a new admin user in the database after validating and hashing the password.
// Returns the created Admin object and any error encountered.
func (fsl *FsLite) insertAdmin(username, password string) (Admin, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("[FSL_ADMIN_insert] failed to retrieve db conn: %v", err)

		return Admin{}, fmt.Errorf("failed to retrieve db conn: %w", err)
	}

	id := uuid.New()
	admin := Admin{
		ID:       id,
		Username: username,
		Password: password,
	}
	err = admin.validate()
	if err != nil {
		log.Printf("[FSL_ADMIN_insert] failed to validate the user: %v", err)

		return Admin{}, fmt.Errorf("failed to validate the user: %w", err)
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

		return Admin{}, fmt.Errorf("failed to hash the pass: %w", err)
	}
	admin.Password = string(hashpass)

	_, err = db.ExecContext(context.Background(), query, id, username, hashpass)
	if err != nil {
		log.Printf("[FSL_ADMIN_insert] failed to execute query: %v", err)
	}

	return admin, err
}

// authenticateAdmin authenticates an admin user by username and password.
// If successful, returns a signed JWT token.
func (fsl *FsLite) authenticateAdmin(username, password string) (string, error) {
	db, err := fsl.dbh.GetConn()
	if err != nil {
		log.Printf("[FSL_ADMIN_auth] failed to retrieve db conn: %v", err)

		return "", err
	}
	query := `SELECT * FROM user_admin WHERE username = ?`

	admin := Admin{}
	err = db.QueryRowContext(context.Background(), query, username).Scan(admin.ptrFields()...)
	if err != nil {
		log.Printf("[FSL_ADMIN_auth] failed to query and scan correctly: %v", err)

		return "", err
	}

	err = verifyPass(admin.Password, password)
	if err != nil {
		log.Printf("[FSL_ADMIN_auth] password didn't match: %v", err)

		return "", err
	}

	token, err := fsl.tokens.issue(admin.ID.String(), admin.Username)
	if err != nil {
		log.Printf("[FSL_ADMIN_auth] failed generating jwt token: %v", err)

		return "", err
	}

	return token, nil
}

// hash generates a bcrypt hash from the provided password bytes.
func hash(password []byte) ([]byte, error) {
	return bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
}

// verifyPass compares a bcrypt hashed password with its possible plaintext equivalent.
// Returns an error if the passwords do not match.
func verifyPass(hashedPass, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hashedPass), []byte(password)); err != nil {
		return fmt.Errorf("failed to verify pass: %w", err)
	}

	return nil
}

// CustomClaims defines the custom JWT claims used for admin authentication.
// jwt
type CustomClaims struct {
	ID       string `json:"userId"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// tokenSigner issues and checks the standalone server's admin tokens: HS256,
// issuer "fslite", with an expiry. The key is derived from the configured
// JWT secret (it used to be the constant "r4nd0m"); an empty key signs and
// accepts nothing.
type tokenSigner struct {
	key      []byte
	validity time.Duration
}

// hoursToDuration converts JWT_VALIDITY_HOURS (4h when unset). It used to be
// time.Duration(hours) hours, truncating 0.5 to 0: tokens that were expired
// when issued.
func hoursToDuration(hours float64) time.Duration {
	if hours <= 0 {
		return 4 * time.Hour
	}

	return time.Duration(hours * float64(time.Hour))
}

func (ts tokenSigner) issue(userID, username string) (string, error) {
	if len(ts.key) == 0 {
		return "", errors.New("no JWT secret configured")
	}
	now := time.Now()
	claims := CustomClaims{
		ID:       userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "fslite",
			ExpiresAt: jwt.NewNumericDate(now.Add(ts.validity)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   userID,
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(ts.key)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signed, nil
}

func (ts tokenSigner) verify(tokenString string) (*CustomClaims, error) {
	if len(ts.key) == 0 {
		return nil, errors.New("no JWT secret configured")
	}
	claims := &CustomClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(*jwt.Token) (any, error) {
		return ts.key, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer("fslite"), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}

	return claims, nil
}

// deriveTokenKey turns the configured JWT secret into fslite's own key, so
// fslite admin tokens can never be confused with the identity provider's.
func deriveTokenKey(secret []byte) []byte {
	if len(secret) == 0 {
		return nil
	}
	m := hmac.New(sha256.New, secret)
	m.Write([]byte("fslite admin token v1"))

	return m.Sum(nil)
}
