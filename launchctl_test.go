package main

import (
	"fmt"
	"log"
	nethttp "net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var GITHUB_TOKEN string
var DENO_DEPLOY_TOKEN string

func TestMain(m *testing.M) {
	GITHUB_TOKEN = os.Getenv("GITHUB_TOKEN")
	if GITHUB_TOKEN == "" {
		log.Fatal("Missing required env var GITHUB_TOKEN")
	}
	DENO_DEPLOY_TOKEN = os.Getenv("DENO_DEPLOY_TOKEN")
	if DENO_DEPLOY_TOKEN == "" {
		log.Fatal("Missing required env var DENO_DEPLOY_TOKEN")
	}
	os.Exit(m.Run())
}

func TestCreateGitHubRepository(t *testing.T) {
	t.Parallel()
	PROJECT_NAME := "af-test"
	_, _, err := createGitHubRepository(GITHUB_TOKEN, PROJECT_NAME, "svelte-astro-deno")
	assert.Nil(t, err)
	// Tidy GitHub repository
	req, _ := nethttp.NewRequest(nethttp.MethodDelete, fmt.Sprintf("https://api.github.com/repos/fraserdarwent/%v", PROJECT_NAME), nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", GITHUB_TOKEN))
	_, err = nethttp.DefaultClient.Do(req)
	assert.Nil(t, err)
}

func TestCreateDenoProject(t *testing.T) {
	t.Parallel()
	PROJECT_NAME := "af-test"
	project, err := createDenoProject(DENO_DEPLOY_TOKEN, PROJECT_NAME)
	assert.Nil(t, err)
	// Tidy Deno Deploy project
	req, _ := nethttp.NewRequest(nethttp.MethodDelete, fmt.Sprintf("https://dash.deno.com/_api/projects/%v", project.ID), nil)
	req.Header.Add("Cookie", fmt.Sprintf("token=%s", DENO_DEPLOY_TOKEN))
	_, err = nethttp.DefaultClient.Do(req)
	assert.Nil(t, err)
}

func TestLink(t *testing.T) {
	t.Parallel()
	PROJECT_NAME := "af-test-int"
	repository, user, err := createGitHubRepository(GITHUB_TOKEN, PROJECT_NAME, "svelte-astro-deno")
	assert.Nil(t, err)
	project, err := createDenoProject(DENO_DEPLOY_TOKEN, fmt.Sprintf("%v-%v", user.Login, PROJECT_NAME))
	assert.Nil(t, err)
	err = link(GITHUB_TOKEN, DENO_DEPLOY_TOKEN, repository, user, project)
	assert.Nil(t, err)

	// Tidy GitHub repository
	req, _ := nethttp.NewRequest(nethttp.MethodDelete, fmt.Sprintf("https://api.github.com/repos/fraserdarwent/%v", PROJECT_NAME), nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", GITHUB_TOKEN))
	_, err = nethttp.DefaultClient.Do(req)
	assert.Nil(t, err)

	// Tidy Deno Deploy project
	req, _ = nethttp.NewRequest(nethttp.MethodDelete, fmt.Sprintf("https://dash.deno.com/_api/projects/%v", project.ID), nil)
	req.Header.Add("Cookie", fmt.Sprintf("token=%s", DENO_DEPLOY_TOKEN))
	_, err = nethttp.DefaultClient.Do(req)
	assert.Nil(t, err)
}
