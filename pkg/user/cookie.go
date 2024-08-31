package user

import (
	"net/http"
	"strings"
)

type cookieExists struct {
	name string
}

func CookieExists(name string) User {
	return &cookieExists{name: strings.ToLower(name)}
}

func (u *cookieExists) IsUser(r *http.Request) bool {
	for _, cookie := range r.Cookies() {
		if strings.ToLower(cookie.Name) == u.name {
			return true
		}
	}
	return false
}
func (u *cookieExists) String() string {
	return "cookie.exists = " + u.name
}
