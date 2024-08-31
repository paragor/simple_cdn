package user

import (
	"fmt"
	"net/http"
	"regexp"
)

type userAgent struct {
	re *regexp.Regexp
}

func (u *userAgent) IsUser(r *http.Request) bool {
	return u.re.MatchString(r.Header.Get("User-Agent"))
}

func UserAgentPattern(pattern string) (User, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}
	return &userAgent{re: re}, nil
}
func (u *userAgent) String() string {
	return "user_agent.pattern = " + u.re.String()
}
