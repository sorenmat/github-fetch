package main

import (
	"testing"

	"github.com/magiconair/properties/assert"
)

func Test_ownerAndRepo(t *testing.T) {
	owner, repo := ownerAndRepo("git@github.com:owner/repo.git")
	assert.Equal(t, "owner", owner)
	assert.Equal(t, "repo", repo)
}
