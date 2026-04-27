package model

type Role struct {
	ID          uint         `json:"id" gorm:"primary_key"`
	Title       string       `json:"title"`
	Permissions []Permission `json:"-" gorm:"many2many:role_permissions"`
}
