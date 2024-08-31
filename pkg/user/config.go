package user

import (
	"fmt"
)

type Config struct {
	Any    []Config `yaml:"any,omitempty"`
	And    []Config `yaml:"and,omitempty"`
	Not    *Config  `yaml:"not,omitempty"`
	Cookie *struct {
		Exists *string `yaml:"exists,omitempty"`
	} `yaml:"cookie,omitempty"`
	UserAgent *struct {
		Pattern *string `yaml:"pattern,omitempty"`
	} `yaml:"user_agent,omitempty"`
	Header *struct {
		Exists  *string `yaml:"exists,omitempty"`
		Pattern *struct {
			Name    string `yaml:"name"`
			Pattern string `yaml:"pattern"`
		} `yaml:"pattern,omitempty"`
	} `yaml:"header,omitempty"`
	Query *struct {
		Count *struct {
			Gte int `yaml:"gte"`
			Lte int `yaml:"lte"`
		} `yaml:"count"`
	} `yaml:"query"`
	Path *struct {
		Pattern *string `yaml:"pattern,omitempty"`
	} `yaml:"path,omitempty"`
	Always *bool `yaml:"always,omitempty"`
	Never  *bool `yaml:"never,omitempty"`
}

func (c *Config) ToUser() User {
	if c.Always != nil {
		return Always()
	}
	if c.Never != nil {
		return Never()
	}
	if c.Not != nil {
		return Not(c.Not.ToUser())
	}
	if c.UserAgent != nil {
		if c.UserAgent.Pattern != nil {
			return must(UserAgentPattern(*c.UserAgent.Pattern))
		}
	}
	if c.Cookie != nil {
		if c.Cookie.Exists != nil {
			return CookieExists(*c.Cookie.Exists)
		}
	}
	if c.Header != nil {
		if c.Header.Exists != nil {
			return HeaderExists(*c.Header.Exists)
		}
		if c.Header.Pattern != nil {
			return must(HeaderPattern(c.Header.Pattern.Name, c.Header.Pattern.Pattern))
		}
	}
	if c.And != nil {
		users := []User{}
		for _, subconfig := range c.And {
			users = append(users, subconfig.ToUser())
		}
		return And(users...)
	}
	if c.Any != nil {
		users := []User{}
		for _, subconfig := range c.Any {
			users = append(users, subconfig.ToUser())
		}
		return Any(users...)
	}
	if c.Query != nil {
		if c.Query.Count != nil {
			return must(QueryCount(c.Query.Count.Gte, c.Query.Count.Lte))
		}
	}
	if c.Path != nil {
		if c.Path.Pattern != nil {
			return must(PathPattern(*c.Path.Pattern))
		}
	}

	panic("config to user: empty config")
}

func must[T any](value T, err error) T {
	if err != nil {
		panic(fmt.Errorf("user: %w", err))
	}
	return value
}

func (c *Config) Validate() error {
	foundField := ""
	if c.Any != nil {
		field := "any"
		foundField = field

		for _, item := range c.Any {
			if err := item.Validate(); err != nil {
				return fmt.Errorf("%s: %w", field, err)
			}
		}
	}
	if c.Always != nil {
		field := "always"
		if len(foundField) > 0 {
			return fmt.Errorf("need specify only 1 field, found 2: %s and %s", field, foundField)
		}
		foundField = field
	}
	if c.Not != nil {
		field := "not"
		if len(foundField) > 0 {
			return fmt.Errorf("need specify only 1 field, found 2: %s and %s", field, foundField)
		}
		foundField = field

		if err := c.Not.Validate(); err != nil {
			return fmt.Errorf("not: %w", err)
		}
	}
	if c.Never != nil {
		field := "never"
		if len(foundField) > 0 {
			return fmt.Errorf("need specify only 1 field, found 2: %s and %s", field, foundField)
		}
		foundField = field
	}

	if c.And != nil {
		field := "and"
		if len(foundField) > 0 {
			return fmt.Errorf("need specify only 1 field, found 2: %s and %s", field, foundField)
		}
		foundField = field

		for _, item := range c.And {
			if err := item.Validate(); err != nil {
				return fmt.Errorf("%s: %w", field, err)
			}
		}
	}
	if c.Cookie != nil {
		field := "cookie"
		if len(foundField) > 0 {
			return fmt.Errorf("need specify only 1 field, found 2: %s and %s", field, foundField)
		}
		foundField = field

		optionFound := false
		if c.Cookie.Exists != nil && len(*c.Cookie.Exists) != 0 {
			optionFound = true
		}
		if !optionFound {
			return fmt.Errorf("field %s required set 'exists' field", field)
		}
	}
	if c.Header != nil {
		field := "header"
		if len(foundField) > 0 {
			return fmt.Errorf("need specify only 1 field, found 2: %s and %s", field, foundField)
		}
		foundField = field

		optionFound := false
		if c.Header.Exists != nil && len(*c.Header.Exists) != 0 {
			optionFound = true
		}
		if c.Header.Pattern != nil && len(c.Header.Pattern.Name) != 0 {
			optionFound = true
			if _, err := HeaderPattern(c.Header.Pattern.Name, c.Header.Pattern.Pattern); err != nil {
				return fmt.Errorf("field %s: %w", field, err)
			}
		}
		if !optionFound {
			return fmt.Errorf("field %s required set 'exists' field", field)
		}
	}
	if c.UserAgent != nil {
		field := "user_agent"
		if len(foundField) > 0 {
			return fmt.Errorf("need specify only 1 field, found 2: %s and %s", field, foundField)
		}
		foundField = field

		optionFound := false
		if c.UserAgent.Pattern != nil && len(*c.UserAgent.Pattern) != 0 {
			optionFound = true
			if _, err := UserAgentPattern(*c.UserAgent.Pattern); err != nil {
				return fmt.Errorf("field %s: %w", field, err)
			}
		}
		if !optionFound {
			return fmt.Errorf("field %s required set 'pattern' field", field)
		}
	}
	if c.Query != nil {
		field := "query"
		if len(foundField) > 0 {
			return fmt.Errorf("need specify only 1 field, found 2: %s and %s", field, foundField)
		}
		foundField = field

		optionFound := false
		if c.Query.Count != nil {
			optionFound = true
			if _, err := QueryCount(c.Query.Count.Gte, c.Query.Count.Lte); err != nil {
				return fmt.Errorf("field %s: %w", field, err)
			}
		}
		if !optionFound {
			return fmt.Errorf("field %s required set 'count' field", field)
		}
	}
	if c.Path != nil {
		field := "path"
		if len(foundField) > 0 {
			return fmt.Errorf("need specify only 1 field, found 2: %s and %s", field, foundField)
		}
		foundField = field

		optionFound := false
		if c.Path.Pattern != nil && len(*c.Path.Pattern) != 0 {
			optionFound = true
			if _, err := PathPattern(*c.Path.Pattern); err != nil {
				return fmt.Errorf("field %s: %w", field, err)
			}
		}
		if !optionFound {
			return fmt.Errorf("field %s required set 'pattern' field", field)
		}
	}

	if len(foundField) == 0 {
		return fmt.Errorf("empty config")
	}
	return nil
}
