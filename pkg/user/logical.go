package user

import (
	"net/http"
	"strings"
)

type anyChain struct {
	users []User
}

func Any(users ...User) User {
	return &anyChain{users: users}
}

func (u *anyChain) IsUser(r *http.Request) bool {
	for _, user := range u.users {
		if user.IsUser(r) {
			return true
		}
	}
	return false
}

func (u *anyChain) String() string {
	children := make([]string, 0, len(u.users))
	for _, user := range u.users {
		children = append(children, addIndentToOutput(user.String()))
	}
	return "any = \n" + strings.Join(children, "\n")
}

type andChain struct {
	users []User
}

func And(users ...User) User {
	return &andChain{users: users}
}

func (u *andChain) IsUser(r *http.Request) bool {
	for _, user := range u.users {
		if !user.IsUser(r) {
			return false
		}
	}
	return true
}

func (u *andChain) String() string {
	children := make([]string, 0, len(u.users))
	for _, user := range u.users {
		children = append(children, addIndentToOutput(user.String()))
	}
	return "and = \n" + strings.Join(children, "\n")
}

type not struct {
	user User
}

func Not(user User) User {
	return &not{user: user}
}

func (u *not) IsUser(r *http.Request) bool {
	return !u.user.IsUser(r)
}
func (u *not) String() string {
	return "not = \n" + addIndentToOutput(u.user.String())
}

type always struct {
}

func Always() User {
	return &always{}
}

func (u *always) IsUser(_ *http.Request) bool {
	return true
}

func (u *always) String() string {
	return "always"
}

type never struct {
}

func Never() User {
	return &never{}
}

func (u *never) String() string {
	return "never"
}

func (u *never) IsUser(_ *http.Request) bool {
	return false
}

func addIndentToOutput(str string) string {
	lines := strings.Split(str, "\n")
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}
