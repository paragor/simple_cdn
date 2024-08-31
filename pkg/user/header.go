package user

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

type headerExists struct {
	name string
}

func HeaderExists(name string) User {
	return &headerExists{name: strings.ToLower(name)}
}

func (u *headerExists) IsUser(r *http.Request) bool {
	for header := range r.Header {
		if u.name == strings.ToLower(header) {
			return true
		}
	}
	return false
}
func (u *headerExists) String() string {
	return "header.exists = " + u.name
}

type headerPattern struct {
	name string
	re   *regexp.Regexp
}

func HeaderPattern(name string, pattern string) (User, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}
	return &headerPattern{name: strings.ToLower(name), re: re}, nil
}

func (u *headerPattern) IsUser(r *http.Request) bool {
	for header := range r.Header {
		if u.name == strings.ToLower(header) && u.re.MatchString(r.Header.Get(u.name)) {
			return true
		}
	}
	return false
}
func (u *headerPattern) String() string {
	return fmt.Sprintf("header.pattern = %s match '%s'", u.name, u.re.String())
}
