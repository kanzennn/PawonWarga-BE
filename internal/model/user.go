package model

type User struct {
	BaseModel
	Name              string  `gorm:"size:100;not null"              json:"name"`
	Email             string  `gorm:"size:255;uniqueIndex;not null"  json:"email"`
	Password          string  `gorm:"not null"                       json:"-"`
	ProfilePicture    *string `gorm:"size:500"                       json:"profile_picture"`
	ProfilePictureKey *string `gorm:"size:500"                       json:"-"`
}
