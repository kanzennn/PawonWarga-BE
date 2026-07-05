package model

type User struct {
	BaseModel
	Name     string `gorm:"size:100;not null"              json:"name"`
	Email    string `gorm:"size:255;uniqueIndex;not null"  json:"email"`
	Password string `gorm:"not null"                       json:"-"`
	// ProfilePicture stores the object storage KEY (e.g. "profiles/12/abc123.jpg"),
	// not a URL — hidden from JSON on purpose. The public URL is resolved fresh
	// from the current storage config at response time (see service.UserView),
	// so switching storage providers doesn't leave stale links in the database.
	ProfilePicture *string `gorm:"size:500" json:"-"`
}
