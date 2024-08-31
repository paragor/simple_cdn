package user

import (
	"fmt"
	"net/http"
)

type queryCount struct {
	gte int
	lte int
}

func QueryCount(gte int, lte int) (User, error) {
	if lte < gte {
		return nil, fmt.Errorf("should be lte >= gte")
	}
	return &queryCount{gte: gte, lte: lte}, nil
}

func (u *queryCount) IsUser(r *http.Request) bool {
	count := len(r.URL.Query())
	return count >= u.gte && count <= u.lte
}
func (u *queryCount) String() string {
	return fmt.Sprintf("query.count = [%d, %d]", u.gte, u.lte)
}
