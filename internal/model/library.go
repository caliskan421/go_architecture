package model

type Library struct {
	ID    uint   `json:"id" gorm:"primary_key;auto_increment"`
	Name  string `json:"name" gorm:"text"`
	Books []Book `json:"books" gorm:"many2many:library_books;"`
}
