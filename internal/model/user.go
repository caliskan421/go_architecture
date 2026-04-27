package model

import "gorm.io/gorm"

type User struct {
	gorm.Model
	RoleID    uint   `json:"roleID"`
	Role      Role   `json:"role" gorm:"foreignKey:RoleID"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Password  string `json:"-"`
}
