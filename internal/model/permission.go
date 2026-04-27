package model

type Permission struct {
	ID         uint   `json:"id" gorm:"primary_key"`
	Permission string `json:"permission"`
}
