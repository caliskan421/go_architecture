package model

type Book struct {
	ID          uint   `json:"id" gorm:"primary_key;auto_increment"`
	AuthorID    uint   `json:"author_id"`
	Author      Author `json:"author" gorm:"foreignKey:AuthorID"`
	Name        string `json:"name" gorm:"unique"`
	Description string `json:"description" gorm:"text"`
}
