package security

import (
	"encoding/json"
	"io"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

const AllUser = "*"

// Creds is used to parse credential entries from JSON.
type Creds struct {
	Username    string   `json:"username,omitempty"`
	Password    string   `json:"password,omitempty"`
	Permissions []string `json:"perms,omitempty"`
}

// AuthStore is used to handle different permissions of users. It adds very
// basic authentication using the authorization header in HTTP requests.
type AuthStore struct {
	passwords map[string][]byte

	// use struct{} since it's more efficient compared to bool
	permissions map[string]map[string]struct{}
}

// NewAuthStore returns an AuthStore pointer with the maps initialized.
func NewAuthStore() *AuthStore {
	return &AuthStore{
		passwords:   make(map[string][]byte),
		permissions: make(map[string]map[string]struct{}),
	}
}

// Initialize reads JSON from a given io.Reader and populates the fields
// of AuthStore in the process of reading the file.
func (as *AuthStore) Initialize(r io.Reader) error {
	decoder := json.NewDecoder(r)
	if _, err := decoder.Token(); err != nil {
		return err
	}

	var cred Creds
	for decoder.More() {
		if err := decoder.Decode(&cred); err != nil {
			return err
		}

		as.passwords[cred.Username] = []byte(cred.Password)
		as.permissions[cred.Username] = make(map[string]struct{}, len(cred.Permissions))
		for _, perm := range cred.Permissions {
			as.permissions[cred.Username][perm] = struct{}{}
		}
	}

	if _, err := decoder.Token(); err != nil {
		return err
	}

	return nil
}

func (as *AuthStore) SetForAllUser(perms ...string) {
	as.passwords[AllUser] = nil
	for _, perm := range perms {
		as.permissions[AllUser][perm] = struct{}{}
	}
}

// ValidatePassword checks the passwords lists and compares the bcrypt hash of
// given password and the one in stored in the AuthStore.
func (as *AuthStore) ValidatePassword(username string, password []byte) bool {
	pass, ok := as.passwords[username]
	if !ok {
		return false
	}

	return bcrypt.CompareHashAndPassword(pass, password) == nil
}

// CheckReq reads the auth information from the request header and
// checks if the password matches.
func (as *AuthStore) CheckReq(r *http.Request) bool {
	// read the authorization header of the HTTP request.
	username, password, ok := r.BasicAuth()
	return ok && as.ValidatePassword(username, []byte(password))
}

// HasPermisson checks the store for a user's permissions and checks if that
// permission list contains the given permission.
func (as *AuthStore) HasPermisson(username, permission string) bool {
	if mp, ok := as.permissions[username]; ok {
		if _, ok := mp[permission]; ok {
			return true
		}
	}

	return false
}

// HasPermissionReq is a helper to directly check the HTTP request for the
// given permission using the more generic as.HasPermission() function.
func (as *AuthStore) HasPermissionReq(r *http.Request, perm string) bool {
	username, _, ok := r.BasicAuth()
	if !ok {
		return false
	}

	return as.HasPermisson(username, perm)
}
