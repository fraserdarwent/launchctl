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
var CLOUDFLARE_ACCOUNT_ID string
var CLOUDFLARE_API_TOKEN string

func TestMain(m *testing.M) {
	GITHUB_TOKEN = os.Getenv("GITHUB_TOKEN")
	if GITHUB_TOKEN == "" {
		log.Fatal("Missing required env var GITHUB_TOKEN")
	}
	DENO_DEPLOY_TOKEN = os.Getenv("DENO_DEPLOY_TOKEN")
	if DENO_DEPLOY_TOKEN == "" {
		log.Fatal("Missing required env var DENO_DEPLOY_TOKEN")
	}
	CLOUDFLARE_ACCOUNT_ID = os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	if CLOUDFLARE_ACCOUNT_ID == "" {
		log.Fatal("Missing required env var CLOUDFLARE_ACCOUNT_ID")
	}
	CLOUDFLARE_API_TOKEN = os.Getenv("CLOUDFLARE_API_TOKEN")
	if CLOUDFLARE_API_TOKEN == "" {
		log.Fatal("Missing required env var CLOUDFLARE_API_TOKEN")
	}
	os.Exit(m.Run())
}

func TestCreateGitHubRepository(t *testing.T) {
	t.Parallel()
	PROJECT_NAME := "lctl-cgh"
	_, _, err := createGitHubRepository(GITHUB_TOKEN, PROJECT_NAME, "svelte-astro-deno")
	assert.Nil(t, err)
	// Tidy GitHub repository
	req, _ := nethttp.NewRequest(nethttp.MethodDelete, fmt.Sprintf("https://api.github.com/repos/fraserdarwent/%v", PROJECT_NAME), nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", GITHUB_TOKEN))
	res, err := nethttp.DefaultClient.Do(req)
	assert.Nil(t, err)
	assert.Equal(t, 204, res.StatusCode)
}

func TestCreateDenoProject(t *testing.T) {
	t.Parallel()
	PROJECT_NAME := "lctl-cd"
	project, err := createDenoProject(DENO_DEPLOY_TOKEN, PROJECT_NAME)
	assert.Nil(t, err)
	// Tidy Deno Deploy project
	req, _ := nethttp.NewRequest(nethttp.MethodDelete, fmt.Sprintf("https://dash.deno.com/_api/projects/%v", project.ID), nil)
	req.Header.Add("Cookie", fmt.Sprintf("token=%s", DENO_DEPLOY_TOKEN))
	res, err := nethttp.DefaultClient.Do(req)
	assert.Nil(t, err)
	assert.Equal(t, 200, res.StatusCode)
}

func TestCreateCloudflarePagesProject(t *testing.T) {
	t.Parallel()
	// Create Cloudflare Pages project
	projectName := "lctl-ccp"
	err := createCloudflarePagesProject(CLOUDFLARE_ACCOUNT_ID, CLOUDFLARE_API_TOKEN, projectName)
	assert.Nil(t, err)
	// Tidy Cloudflare Pages project
	req, _ := nethttp.NewRequest(nethttp.MethodDelete, fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%v/pages/projects/%v", CLOUDFLARE_ACCOUNT_ID, projectName), nil)
	req.Header.Add("Authorization", "Bearer "+CLOUDFLARE_API_TOKEN)
	res, err := nethttp.DefaultClient.Do(req)
	assert.Nil(t, err)
	assert.Equal(t, 200, res.StatusCode)
}

func TestLinkDenoProject(t *testing.T) {
	t.Parallel()
	PROJECT_NAME := "lctl-ldp"
	repository, user, err := createGitHubRepository(GITHUB_TOKEN, PROJECT_NAME, "svelte-astro-deno")
	assert.Nil(t, err)
	project, err := createDenoProject(DENO_DEPLOY_TOKEN, fmt.Sprintf("%v-%v", user.Login, PROJECT_NAME))
	assert.Nil(t, err)
	err = linkDenoDeployProject(GITHUB_TOKEN, DENO_DEPLOY_TOKEN, repository, user, project)
	assert.Nil(t, err)
	// Tidy GitHub repository
	req, _ := nethttp.NewRequest(nethttp.MethodDelete, fmt.Sprintf("https://api.github.com/repos/fraserdarwent/%v", PROJECT_NAME), nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", GITHUB_TOKEN))
	res, err := nethttp.DefaultClient.Do(req)
	assert.Nil(t, err)
	assert.Equal(t, 204, res.StatusCode)
	// Tidy Deno Deploy project
	req, _ = nethttp.NewRequest(nethttp.MethodDelete, fmt.Sprintf("https://dash.deno.com/_api/projects/%v", project.ID), nil)
	req.Header.Add("Cookie", fmt.Sprintf("token=%s", DENO_DEPLOY_TOKEN))
	res, err = nethttp.DefaultClient.Do(req)
	assert.Nil(t, err)
	assert.Equal(t, 200, res.StatusCode)
}

func TestLinkCloudflarePagesProject(t *testing.T) {
	t.Parallel()
	projectName := "lctl-lcp"
	// Create GitHub repository
	repository, user, err := createGitHubRepository(GITHUB_TOKEN, projectName, "svelte-astro-deno")
	assert.Nil(t, err)
	// Create GitHub repository secrets
	err = createGitHubRepositorySecret(GITHUB_TOKEN, user.Login, repository.name, "CLOUDFLARE_API_TOKEN", CLOUDFLARE_API_TOKEN)
	assert.Nil(t, err)
	err = createGitHubRepositorySecret(GITHUB_TOKEN, user.Login, repository.name, "CLOUDFLARE_ACCOUNT_ID", CLOUDFLARE_ACCOUNT_ID)
	assert.Nil(t, err)
	// Create Cloudflare Pages project
	err = createCloudflarePagesProject(CLOUDFLARE_ACCOUNT_ID, CLOUDFLARE_API_TOKEN, projectName)
	assert.Nil(t, err)
	err = linkCloudflarePagesProject(GITHUB_TOKEN, repository)
	assert.Nil(t, err)
	// Tidy Cloudflare Pages project
	req, _ := nethttp.NewRequest(nethttp.MethodDelete, fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%v/pages/projects/%v", CLOUDFLARE_ACCOUNT_ID, projectName), nil)
	req.Header.Add("Authorization", "Bearer "+CLOUDFLARE_API_TOKEN)
	res, err := nethttp.DefaultClient.Do(req)
	assert.Nil(t, err)
	assert.Equal(t, 200, res.StatusCode)
	// Tidy GitHub repository
	req, _ = nethttp.NewRequest(nethttp.MethodDelete, fmt.Sprintf("https://api.github.com/repos/fraserdarwent/%v", projectName), nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", GITHUB_TOKEN))
	res, err = nethttp.DefaultClient.Do(req)
	assert.Nil(t, err)
	assert.Equal(t, 204, res.StatusCode)
}
