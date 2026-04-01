package user

// UserResponse is the JSON response envelope for a user.
type UserResponse struct {
	ID        string   `json:"id"`
	Username  string   `json:"username"`
	Email     string   `json:"email"`
	FirstName string   `json:"firstName"`
	LastName  string   `json:"lastName"`
	Enabled   bool     `json:"enabled"`
	Roles     []string `json:"roles"`
}

// ResponseFrom converts a User entity to UserResponse.
func ResponseFrom(u User) UserResponse {
	roles := u.Roles
	if roles == nil {
		roles = []string{}
	}
	return UserResponse{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Enabled:   u.Enabled,
		Roles:     roles,
	}
}

// CreateUserRequest is the JSON body for user creation.
type CreateUserRequest struct {
	Username  string   `json:"username"`
	Email     string   `json:"email"`
	FirstName string   `json:"firstName"`
	LastName  string   `json:"lastName"`
	Password  string   `json:"password"`
	Roles     []string `json:"roles"`
}

// UpdateUserRequest is the JSON body for partial user updates.
type UpdateUserRequest struct {
	Email     *string `json:"email,omitempty"`
	FirstName *string `json:"firstName,omitempty"`
	LastName  *string `json:"lastName,omitempty"`
	Enabled   *bool   `json:"enabled,omitempty"`
}
