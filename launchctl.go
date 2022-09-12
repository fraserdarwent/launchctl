package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	nethttp "net/http"
	"os"
	"text/template"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
)

type DenoAPIError struct {
	Message string `json:"message"`
}
type DenoOrganisation struct {
	ID string `json:"id"`
}

type DenoProject struct {
	ID   string `json:"id"`
	name string
}
type GitHubUser struct {
	Login string `json:"login"`
}

type GitHubRepository struct {
	name       string
	repository git.Repository
}

func link(gitHubToken, denoToken string, repository GitHubRepository, gitHubUser GitHubUser, project DenoProject) error {
	println("🔗 Linking")
	// Link the Deno Deploy project
	reqBody := []byte(fmt.Sprintf(`{"organization":"%v","repo":"%v","productionBranch":"main","projectId":"%v"}`, gitHubUser.Login, repository.name, project.ID))
	req, _ := nethttp.NewRequest(nethttp.MethodPost, "https://dash.deno.com/_api/github/link", bytes.NewBuffer(reqBody))
	req.Header.Add("Cookie", fmt.Sprintf("token=%s", denoToken))
	req.Header.Add("Content-Type", "application/json")
	res, err := nethttp.DefaultClient.Do(req)
	if err != nil {
		println(err.Error())
		return errors.New("failed to link Deno Deploy project")
	}
	if 299 < res.StatusCode {
		apiError := DenoAPIError{}
		json.NewDecoder(res.Body).Decode(&apiError)
		println(apiError.Message)
		return errors.New("failed to link Deno Deploy project")

	}

	err = repository.repository.Push(&git.PushOptions{RemoteName: "new", Auth: &http.BasicAuth{Username: "PAT", Password: gitHubToken}})
	if err != nil {
		println(err.Error())
		return errors.New("failed to link Deno Deploy project")
	}

	return nil
}

func createDenoProject(token, name string) (DenoProject, error) {
	println("🛠️ Creating project")
	// Get Deno Deploy organizations
	organisations := []DenoOrganisation{}
	req, _ := nethttp.NewRequest(nethttp.MethodGet, "https://dash.deno.com/_api/organizations", nil)
	req.Header.Add("Cookie", fmt.Sprintf("token=%s", token))
	res, err := nethttp.DefaultClient.Do(req)
	if err != nil {
		println(err.Error())
		return DenoProject{}, errors.New("failed to discover Deno Deploy organisation")
	}
	err = json.NewDecoder(res.Body).Decode(&organisations)
	if err != nil {
		println(err.Error())
		return DenoProject{}, errors.New("failed to discover Deno Deploy organisation")
	}
	if 1 < len(organisations) {
		return DenoProject{}, fmt.Errorf("we only support 1 organisation but found %v", len(organisations))
	}

	// Create the Deno Deploy project
	organisation := organisations[0]

	reqBody := []byte(fmt.Sprintf(`{"name":"%v","organizationId":"%v"}`, name, organisation.ID))
	req, _ = nethttp.NewRequest(nethttp.MethodPost, "https://dash.deno.com/_api/projects", bytes.NewBuffer(reqBody))
	req.Header.Add("Cookie", fmt.Sprintf("token=%s", token))
	req.Header.Add("Content-Type", "application/json")
	res, err = nethttp.DefaultClient.Do(req)
	if err != nil {
		println(err.Error())
		return DenoProject{}, errors.New("failed to create Deno Deploy project")
	}

	if 299 < res.StatusCode {
		apiError := DenoAPIError{}
		json.NewDecoder(res.Body).Decode(&apiError)
		println(apiError.Message)
		return DenoProject{}, errors.New("failed to create Deno Deploy project")
	}

	project := DenoProject{name: name}
	json.NewDecoder(res.Body).Decode(&project)
	return project, nil
}

func createGitHubRepository(token, name, flavour string) (GitHubRepository, GitHubUser, error) {
	println("🏗️ Creating repository")
	repository, err := git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{
		URL: fmt.Sprintf("https://github.com/fraserdarwent/create-%v-app", flavour),
	})

	if err != nil {
		if err.Error() == "authentication required" {
			return GitHubRepository{}, GitHubUser{}, errors.New("unknown flavour")
		}
		return GitHubRepository{}, GitHubUser{}, err
	}

	req, _ := nethttp.NewRequest(nethttp.MethodGet, "https://api.github.com/user", nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	res, err := nethttp.DefaultClient.Do(req)
	if err != nil {
		println(err.Error())
		return GitHubRepository{}, GitHubUser{}, errors.New("failed to discover GitHub user")
	}

	gitHubUser := GitHubUser{}
	json.NewDecoder(res.Body).Decode(&gitHubUser)

	reqBody := []byte(fmt.Sprintf(`{"name": "%v","private": true}`, name))
	req, _ = nethttp.NewRequest(nethttp.MethodPost, "https://api.github.com/user/repos", bytes.NewBuffer(reqBody))
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	_, err = nethttp.DefaultClient.Do(req)
	if err != nil {
		println(err.Error())
		return GitHubRepository{}, gitHubUser, errors.New("failed to create GitHub repository")
	}

	if 299 < res.StatusCode {
		return GitHubRepository{}, gitHubUser, fmt.Errorf("failed to create GitHub repository with %v", res.StatusCode)
	}

	_, err = repository.CreateRemote(&config.RemoteConfig{Name: "new", URLs: []string{fmt.Sprintf("https://github.com/%v/%v", gitHubUser.Login, name)}})
	if err != nil {
		println(err.Error())
		return GitHubRepository{}, gitHubUser, errors.New("failed to configure GitHub remote")
	}

	worktree, err := repository.Worktree()
	if err != nil {
		println(err.Error())
		return GitHubRepository{}, gitHubUser, errors.New("failed to get worktree")
	}

	file, err := worktree.Filesystem.OpenFile(".github/workflows/deploy.yaml", os.O_RDWR, os.ModePerm)
	if err != nil {
		println(err.Error())
		return GitHubRepository{}, gitHubUser, errors.New("failed to create GitHub Action file")
	}

	original, err := io.ReadAll(file)
	if err != nil {
		println(err.Error())
		return GitHubRepository{}, gitHubUser, errors.New("failed to read GitHub Action template")
	}

	file.Close()

	tmpl, err := template.New("action").Parse(string(original))
	if err != nil {
		println(err.Error())
		return GitHubRepository{}, gitHubUser, errors.New("failed to parse GitHub Action template")
	}

	file, err = worktree.Filesystem.OpenFile(".github/workflows/deploy.yaml", os.O_WRONLY, os.ModePerm)
	if err != nil {
		println(err.Error())
		return GitHubRepository{}, gitHubUser, errors.New("failed to create GitHub Action file")
	}

	err = tmpl.Execute(file, map[string]string{"Project": fmt.Sprintf("%v-%v", gitHubUser.Login, name)})
	if err != nil {
		println(err.Error())
		return GitHubRepository{}, gitHubUser, errors.New("failed to write GitHub Action")
	}

	worktree.Add(".github/workflows/deploy.yaml")
	worktree.Commit("Templated GitHub Action", &git.CommitOptions{})

	return GitHubRepository{name: name, repository: *repository}, gitHubUser, nil
}

func main() {
	GITHUB_TOKEN := os.Getenv("GITHUB_TOKEN")
	if GITHUB_TOKEN == "" {
		println("❌ Missing required env var GITHUB_TOKEN")
		os.Exit(1)
	}

	DENO_DEPLOY_TOKEN := os.Getenv("DENO_DEPLOY_TOKEN")
	if DENO_DEPLOY_TOKEN == "" {
		println("❌ Missing required env var DENO_DEPLOY_TOKEN")
		os.Exit(1)
	}

	args := os.Args[1:]

	if len(args) < 2 {
		println("📖 Usage launchctl <flavour> <project_name>")
		os.Exit(1)
	}

	println("🚀 Starting")

	repository, user, err := createGitHubRepository(GITHUB_TOKEN, args[1], args[0])
	if err != nil {
		println(fmt.Sprintf("❌ Error, %v", err.Error()))
		os.Exit(1)
	}

	project, err := createDenoProject(DENO_DEPLOY_TOKEN, fmt.Sprintf("%v-%v", user.Login, args[1]))
	if err != nil {
		println(fmt.Sprintf("❌ Error, %v", err.Error()))
		os.Exit(1)
	}

	err = link(GITHUB_TOKEN, DENO_DEPLOY_TOKEN, repository, user, project)
	if err != nil {
		println(fmt.Sprintf("❌ Error, %v", err.Error()))
		os.Exit(1)
	}

	println("🎉 Finished")
	println(fmt.Sprintf("🦕 https://dash.deno.com/projects/%v", project.name))
	println(fmt.Sprintf("🐙 https://github.com/%v/%v", user.Login, repository.name))

}
