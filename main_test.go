package main

import (
	"github.com/paragor/simple_cdn/examples"
	"testing"
)

func TestParseConfig(t *testing.T) {
	files, err := examples.ExampleConfigs.ReadDir(".")
	if err != nil {
		panic(err)
	}
	for _, file := range files {
		t.Run(file.Name(), func(t *testing.T) {
			content, err := examples.ExampleConfigs.ReadFile(file.Name())
			if err != nil {
				panic(err)
			}
			_, err = ParseConfig(content)
			if err != nil {
				t.Errorf("ParseConfig() error = %v", err)
				return
			}
		})
	}
}
