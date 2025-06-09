// Package fslite provides functionality for authentication. Only admin user management,
// registration, authentication, password hashing, and JWT tokens issuing
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
	minimumUserLength = 3
	minimumPassLength = 5
)

var (
	// hashCost defines the bcrypt hashing cost for password hashing.
	hashCost = 4
	// JwtValidityHours specifies the number of hours a JWT token is valid.
	JwtValidityHours float64 = 4
	// jwtSecretKey is the secret key used for signing JWT tokens.
	jwtSecretKey = "r4nd0m"
	// usernameRegex is the regular expression for validating usernames.
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	// passwordRegex is the regular expression for validating passwords.
	passwordRegex = regexp.MustCompile(`^[a-zA-Z0-9!@#\$%\^&\*]+$`)
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

	if verbose {
		log.Printf("[FSL_ADMIN_insert] inserting default user: %+v", admin)
	}

	_, err = db.Exec(query, id, username, hashpass)
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
	err = db.QueryRow(query, username).Scan(admin.ptrFields()...)
	if err != nil {
		log.Printf("[FSL_ADMIN_auth] failed to query and scan correctly: %v", err)

		return "", err
	}

	err = verifyPass(admin.Password, password)
	if err != nil {
		log.Printf("[FSL_ADMIN_auth] password didn't match: %v", err)

		return "", err
	}

	token, err := generateAccessJWT(admin.ID.String(), admin.Username)
	if err != nil {
		log.Printf("[FSL_ADMIN_auth] failed generating jwt token: %v", err)

		return "", err
	}

	return token, nil
}

// hash generates a bcrypt hash from the provided password bytes.
func hash(password []byte) ([]byte, error) {
	return bcrypt.GenerateFromPassword(password, hashCost)
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

// generateAccessJWT creates and signs a JWT token for the given user ID and username.
// Returns the signed token string or an error.
func generateAccessJWT(userID, username string) (string, error) {
	// Set the claims for the token
	claims := CustomClaims{
		ID:       userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "fslite",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * time.Duration(JwtValidityHours))),
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

// decodeJWT parses and validates a JWT token string, returning the claims if valid.
// Returns a boolean indicating validity, the claims, and any error encountered.
func decodeJWT(tokenString string) (bool, *CustomClaims, error) {
	// Parse and validate the token
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(jwtSecretKey), nil
	})
	if err != nil {
		log.Printf("[FSL_ADMIN_decjwt] %v token, exiting", token)

		return false, nil, err
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok {
		log.Printf("[FSL_ADMIN_decjwt] not okay when retrieving claims")

		return false, nil, errors.New("invalid claims")
	}

	return true, claims, nil
}
