package user

import (
	"fmt"
	"net/http"
	"regexp"
)

type pathPattern struct {
	re *regexp.Regexp
}

func (u *pathPattern) IsUser(r *http.Request) bool {
	return u.re.MatchString(r.URL.Path)
}

func PathPattern(pattern string) (User, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}
	return &pathPattern{re: re}, nil
}
func (u *pathPattern) String() string {
	return "path.pattern = " + u.re.String()
}
