package user

import (
	"net/http"
)

type User interface {
	IsUser(r *http.Request) bool
	String() string
}
