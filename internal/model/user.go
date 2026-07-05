package model

import "time"

type User struct {
	BaseModel
	Name        string  `gorm:"size:100;not null"              json:"name"`
	Email       string  `gorm:"size:255;uniqueIndex;not null"  json:"email"`
	Password    string  `gorm:"not null"                       json:"-"`
	PhoneNumber *string `gorm:"size:20"                        json:"phone_number"`
	// ProfilePicture stores the object storage KEY (e.g. "profiles/12/abc123.jpg"),
	// not a URL — hidden from JSON on purpose. The public URL is resolved fresh
	// from the current storage config at response time (see service.UserView),
	// so switching storage providers doesn't leave stale links in the database.
	ProfilePicture *string `gorm:"size:500" json:"-"`
	// TokenValidAfter implements "log out of all devices" without a session
	// table: JWTs are otherwise stateless, so middleware.JWTAuth rejects any
	// token whose IssuedAt is before this timestamp. Nil means no restriction.
	TokenValidAfter *time.Time `json:"-"`
}
