// Package user implements the user management domain via Keycloak Admin API.
package user

// User represents a Keycloak user within a tenant realm.
type User struct {
	ID        string
	Username  string
	Email     string
	FirstName string
	LastName  string
	Enabled   bool
	Roles     []string
}
