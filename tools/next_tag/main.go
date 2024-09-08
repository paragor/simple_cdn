package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"log"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
)

func main() {
	args := os.Args
	if len(args) != 2 {
		panic("only one arg require")
	}
	upgradeType := strings.ToLower(args[1])
	switch upgradeType {
	case "major", "minor", "patch":
	default:
		log.Fatalln("arg[1] should one of: major, minor, patch")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	gitTag := exec.CommandContext(ctx, "git", "tag")
	gitTag.Stdout = stdout
	gitTag.Stderr = stderr
	if err := gitTag.Run(); err != nil {
		log.Fatalf("cant run git tag: %w. stderr: %s", err, stderr.String())
	}
	rawTags := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	tags := []*semver.Version{}
	for _, rawTag := range rawTags {
		if !strings.HasPrefix(rawTag, "v") {
			continue
		}
		rawTag = strings.TrimPrefix(rawTag, "v")
		tag, err := semver.StrictNewVersion(rawTag)
		if err != nil {
			continue
		}
		tags = append(tags, tag)
	}

	slices.SortFunc(tags, func(a, b *semver.Version) int {
		return b.Compare(a)
	})

	newTag := *tags[0]
	if strings.ToLower(args[1]) == "major" {
		newTag = newTag.IncMajor()
	} else if strings.ToLower(args[1]) == "minor" {
		newTag = newTag.IncMinor()
	} else {
		newTag = newTag.IncPatch()
	}
	fmt.Printf("v%s", newTag.String())
}
