package data

import (
	"database/sql"
	"fmt"
	"time"
)

type User struct {
	Id        string
	GithubId  int64
	Email     string
	Login     string
	Role      Role
	CreatedAt time.Time
}

type Role string

const (
	RoleAdmin   Role = "admin"
	RoleAnalyst Role = "analyst"
)

func (r Role) Valid() bool {
	return r == RoleAdmin || r == RoleAnalyst
}

type UserRepo struct {
	DB *sql.DB
}

func (r *UserRepo) IsFirstUser() (bool, error) {
	var count int
	err := r.DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count == 0, err
}

func (r *UserRepo) GetUserByGithubID(githubID int64) (*User, error) {
	u := &User{}
	err := r.DB.QueryRow(
		"SELECT id, github_id, email, login, role, created_at FROM users WHERE github_id = ?",
		githubID,
	).Scan(&u.Id, &u.GithubId, &u.Email, &u.Login, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}
func (r *UserRepo) GetUserByID(id string) (*User, error) {
	u := &User{}
	err := r.DB.QueryRow(
		"SELECT id, github_id, email, login, role, created_at FROM users WHERE id = ?", id,
	).Scan(&u.Id, &u.GithubId, &u.Email, &u.Login, &u.Role, &u.CreatedAt)
	return u, err
}

func (r *UserRepo) CreateUser(u *User) error {
	if !u.Role.Valid() {
		return fmt.Errorf("invalid role: %s", u.Role)
	}
	_, err := r.DB.Exec(
		"INSERT INTO users (id, github_id, email, login, role) VALUES (?, ?, ?, ?, ?)",
		u.Id, u.GithubId, u.Email, u.Login, u.Role,
	)
	return err
}

func (r *UserRepo) PromoteUser(userId string, role Role) error {
	if !role.Valid() {
		return fmt.Errorf("invalid role: %s", role)
	}
	_, err := r.DB.Exec("UPDATE users SET role = ? WHERE id = ?", role, userId)
	return err
}
