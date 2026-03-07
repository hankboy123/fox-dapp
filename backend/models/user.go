package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username string `gorm:"unique;not null;size:50" json:"username"`
	Email    string `gorm:"unique;not null;size:100" json:"email"`
	Password string `gorm:"not null" json:"-"`
}

type CreateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email,max=100"`
	Password string `json:"password" binding:"required,min=6"`
}

type UpdateUserRequest struct {
	Email    *string `json:"email" binding:"omitempty,email,max=100"`
	Password *string `json:"password" binding:"omitempty,min=6"`
}
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email,max=100"`
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6"`
}

type UserResponse struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}
