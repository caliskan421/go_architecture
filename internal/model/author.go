package model

type Author struct {
	ID          uint   `json:"id" gorm:"primary_key;auto_increment"`
	Name        string `json:"name" gorm:"unique"`
	Description string `json:"description" gorm:"text"`
}
