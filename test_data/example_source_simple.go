package main

import (
	"fmt"
	"time"
)

// User represents a simple user
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UserManager manages users
type UserManager struct {
	users map[int]*User
}

// NewUserManager creates a new user manager
func NewUserManager() *UserManager {
	return &UserManager{
		users: make(map[int]*User),
	}
}

// CreateUser creates a new user
func (um *UserManager) CreateUser(id int, name, email string) error {
	if _, exists := um.users[id]; exists {
		return fmt.Errorf("user already exists: %d", id)
	}
	
	um.users[id] = &User{
		ID:    id,
		Name:  name,
		Email: email,
	}
	return nil
}

// GetUser retrieves a user by ID
func (um *UserManager) GetUser(id int) (*User, error) {
	user, exists := um.users[id]
	if !exists {
		return nil, fmt.Errorf("user not found: %d", id)
	}
	return user, nil
}

func main() {
	um := NewUserManager()
	
	// Example usage
	err := um.CreateUser(1, "Alice", "alice@example.com")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	user, err := um.GetUser(1)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	fmt.Printf("User: %+v\n", user)
	fmt.Printf("Current time: %v\n", time.Now())
}
