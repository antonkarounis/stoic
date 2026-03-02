package models

import (
	"time"

	"golang.org/x/oauth2"
)

type SessionData struct {
	Token      *oauth2.Token
	TokenData  []byte    // encrypted token bytes from the database
	IDToken    string
	SubjectID  string    // auth provider subject ID
	IdentityID int64     // identities.id in the database
	UserID     *UserID   // nil if identity not yet linked to a domain user
	Roles      []string
	Expires    time.Time
}
