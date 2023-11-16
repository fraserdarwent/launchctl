package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	nethttp "net/http"
	"os"
	"strings"
	"text/template"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/nacl/box"
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

type GitHubRepositorySecret struct {
	KeyId string `json:"key_id"`
	Key   string `json:"key"`
}

func linkCloudflarePagesProject(gitHubToken string, repository GitHubRepository) error {
	err := repository.repository.Push(&git.PushOptions{RemoteName: "new", Auth: &http.BasicAuth{Username: "PAT", Password: gitHubToken}})
	if err != nil {
		return fmt.Errorf("failed to link Cloudflare Pages project: %v", err)
	}
	return nil
}

func linkDenoDeployProject(gitHubToken, denoToken string, repository GitHubRepository, gitHubUser GitHubUser, project DenoProject) error {
	println("🔗 Linking Deno Deploy project to GitHub repository")
	// Link the Deno Deploy project
	reqBody := []byte(fmt.Sprintf(`{"organization":"%v","repo":"%v","productionBranch":"main","projectId":"%v"}`, gitHubUser.Login, repository.name, project.ID))
	req, _ := nethttp.NewRequest(nethttp.MethodPost, "https://dash.deno.com/_api/github/link", bytes.NewBuffer(reqBody))
	req.Header.Add("Cookie", fmt.Sprintf("token=%s", denoToken))
	req.Header.Add("Content-Type", "application/json")
	res, err := nethttp.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to link Deno Deploy project: %v", err)
	}
	if 299 < res.StatusCode {
		apiError := DenoAPIError{}
		json.NewDecoder(res.Body).Decode(&apiError)
		println(apiError.Message)
		return fmt.Errorf("failed to link Deno Deploy project: %v", apiError)
	}

	err = repository.repository.Push(&git.PushOptions{RemoteName: "new", Auth: &http.BasicAuth{Username: "PAT", Password: gitHubToken}})
	if err != nil {
		return fmt.Errorf("failed to link Deno Deploy project: %v", err)
	}

	return nil
}

func createDenoProject(token, name string) (DenoProject, error) {
	println("🛠️ Creating Deno Deploy project")
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

func createGitHubRepositorySecret(token, userName, repositoryName, name, value string) error {
	println("🤫 Creating GitHub repository secret")
	req, _ := nethttp.NewRequest(nethttp.MethodGet, fmt.Sprintf("https://api.github.com/repos/%v/%v/actions/secrets/public-key", userName, repositoryName), nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	res, err := nethttp.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get repository public key: %v", err)
	}
	if 299 < res.StatusCode {
		return fmt.Errorf("failed to get repository public key: got %v", res.StatusCode)
	}
	secret := GitHubRepositorySecret{}
	err = json.NewDecoder(res.Body).Decode(&secret)
	if err != nil {
		return fmt.Errorf("failed to create GitHub repository secret: %v", err)
	}
	encrypted, err := encryptSecret(secret.Key, value)
	if err != nil {
		return fmt.Errorf("failed to encrypt secret: %v", err)
	}
	reqBody := []byte(fmt.Sprintf(`{"encrypted_value": "%v","key_id": "%v"}`, encrypted, secret.KeyId))
	req, _ = nethttp.NewRequest(nethttp.MethodPut, fmt.Sprintf("https://api.github.com/repos/%v/%v/actions/secrets/%v", userName, repositoryName, name), bytes.NewBuffer(reqBody))
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	res, err = nethttp.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create GitHub repository secret: %v", err)
	}
	if 299 < res.StatusCode {
		return fmt.Errorf("failed to create GitHub repository secret: got %v", res.StatusCode)
	}
	return nil
}

func createGitHubRepository(token, name, flavour string) (GitHubRepository, GitHubUser, error) {
	println("🏗️ Creating GitHub repository")
	repository, err := git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{
		URL: fmt.Sprintf("https://github.com/fraserdarwent/launchctl-%v", flavour),
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
	err = json.NewDecoder(res.Body).Decode(&gitHubUser)
	if err != nil {
		return GitHubRepository{}, GitHubUser{}, fmt.Errorf("failed to discover GitHub user: %v", err)
	}

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

	tmpl, err := template.New("action").Delims(" {{", "}}").Parse(string(original))
	if err != nil {
		println(err.Error())
		return GitHubRepository{}, gitHubUser, errors.New("failed to parse GitHub Action template")
	}

	file, err = worktree.Filesystem.OpenFile(".github/workflows/deploy.yaml", os.O_WRONLY, os.ModePerm)
	if err != nil {
		println(err.Error())
		return GitHubRepository{}, gitHubUser, errors.New("failed to create GitHub Action file")
	}

	err = tmpl.Execute(file, map[string]string{
		"Project": fmt.Sprintf(" %v-%v", gitHubUser.Login, name),
	})
	if err != nil {
		return GitHubRepository{}, gitHubUser, fmt.Errorf("failed to write GitHub Action: %v", err)
	}

	_, err = worktree.Add(".github/workflows/deploy.yaml")
	if err != nil {
		return GitHubRepository{}, gitHubUser, fmt.Errorf("failed to write GitHub Action: %v", err)
	}

	_, err = worktree.Commit("Templated GitHub Action", &git.CommitOptions{})
	if err != nil {
		return GitHubRepository{}, gitHubUser, fmt.Errorf("failed to write GitHub Action: %v", err)
	}

	return GitHubRepository{name: name, repository: *repository}, gitHubUser, nil
}

func main() {
	GITHUB_TOKEN := os.Getenv("GITHUB_TOKEN")
	if GITHUB_TOKEN == "" {
		println("❌ Missing required env var GITHUB_TOKEN")
		os.Exit(1)
	}

	args := os.Args[1:]

	if len(args) < 2 {
		println("📖 Usage launchctl <flavour> <project_name>")
		os.Exit(1)
	}

	println("🚀 Starting v1.1")
	flavour := args[0]
	projectName := args[1]
	if strings.HasSuffix(flavour, "deno") {
		DENO_DEPLOY_TOKEN := os.Getenv("DENO_DEPLOY_TOKEN")
		if DENO_DEPLOY_TOKEN == "" {
			println("❌ Missing required env var DENO_DEPLOY_TOKEN")
			os.Exit(1)
		}

		repository, user, err := createGitHubRepository(GITHUB_TOKEN, projectName, flavour)
		if err != nil {
			println(fmt.Sprintf("❌ Error, %v", err.Error()))
			os.Exit(1)
		}

		project, err := createDenoProject(DENO_DEPLOY_TOKEN, fmt.Sprintf("%v-%v", user.Login, projectName))
		if err != nil {
			println(fmt.Sprintf("❌ Error, %v", err.Error()))
			os.Exit(1)
		}

		err = linkDenoDeployProject(GITHUB_TOKEN, DENO_DEPLOY_TOKEN, repository, user, project)
		if err != nil {
			println(fmt.Sprintf("❌ Error, %v", err.Error()))
			os.Exit(1)
		}

		println("🎉 Finished")
		println(fmt.Sprintf("🦕 https://dash.deno.com/projects/%v", project.name))
		println(fmt.Sprintf("🐙 https://github.com/%v/%v", user.Login, repository.name))
	}
	if strings.HasSuffix(flavour, "cloudflare") {
		CLOUDFLARE_ACCOUNT_ID := os.Getenv("CLOUDFLARE_ACCOUNT_ID")
		if CLOUDFLARE_ACCOUNT_ID == "" {
			println("❌ Missing required env var CLOUDFLARE_ACCOUNT_ID")
			os.Exit(1)
		}
		CLOUDFLARE_API_TOKEN := os.Getenv("CLOUDFLARE_API_TOKEN")
		if CLOUDFLARE_API_TOKEN == "" {
			println("❌ Missing required env var CLOUDFLARE_API_TOKEN")
			os.Exit(1)
		}
		repository, user, err := createGitHubRepository(GITHUB_TOKEN, projectName, flavour)
		if err != nil {
			println(fmt.Sprintf("❌ Error, %v", err.Error()))
			os.Exit(1)
		}
		err = createGitHubRepositorySecret(GITHUB_TOKEN, user.Login, repository.name, "CLOUDFLARE_API_TOKEN", CLOUDFLARE_API_TOKEN)
		if err != nil {
			println(fmt.Sprintf("❌ Error, %v", err.Error()))
			os.Exit(1)
		}
		err = createGitHubRepositorySecret(GITHUB_TOKEN, user.Login, repository.name, "CLOUDFLARE_ACCOUNT_ID", CLOUDFLARE_ACCOUNT_ID)
		if err != nil {
			println(fmt.Sprintf("❌ Error, %v", err.Error()))
			os.Exit(1)
		}
		err = createCloudflarePagesProject(CLOUDFLARE_ACCOUNT_ID, CLOUDFLARE_API_TOKEN, fmt.Sprintf("%v-%v", user.Login, projectName))
		if err != nil {
			println(fmt.Sprintf("❌ Error, %v", err.Error()))
			os.Exit(1)
		}
		err = linkCloudflarePagesProject(GITHUB_TOKEN, repository)
		if err != nil {
			println(fmt.Sprintf("❌ Error, %v", err.Error()))
			os.Exit(1)
		}
		println("🎉 Finished")
		println(fmt.Sprintf("🟠 https://dash.cloudflare.com/%v/pages/view/%v", CLOUDFLARE_ACCOUNT_ID, fmt.Sprintf("%v-%v", user.Login, projectName)))
		println(fmt.Sprintf("🐙 https://github.com/%v/%v", user.Login, repository.name))
	}
}

func createCloudflarePagesProject(accountId, token, name string) error {
	println("🛠️ Creating Cloudflare Pages project")
	reqBody := []byte(fmt.Sprintf(`{
		"build_config": {},
		"canonical_deployment": {},
		"deployment_configs": {
			"preview": {},
			"production": {}
		},
		"latest_deployment": {},
		"name": "%v",
		"production_branch": "main"
	}`, name))
	req, _ := nethttp.NewRequest(nethttp.MethodPost, fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%v/pages/projects", accountId), bytes.NewBuffer(reqBody))
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %v", token))
	res, err := nethttp.DefaultClient.Do(req)
	if err != nil {
		println(err.Error())
		return fmt.Errorf("failed to create Cloudflare Pages project: %v", err)
	}
	if 299 < res.StatusCode {
		return fmt.Errorf("failed to create Cloudflare Pages project: got %v", res.StatusCode)
	}
	return nil
}

const (
	keySize   = 32
	nonceSize = 24
)

var generateKey = box.GenerateKey

// https://zostay.com/posts/2022/05/04/do-not-use-libsodium-with-go/
func encryptSecret(publicKey, secret string) (string, error) {
	// decode the provided public key from base64
	recipientKey := new([keySize]byte)
	b, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil {
		return "", err
	} else if size := len(b); size != keySize {
		return "", fmt.Errorf("recipient public key has invalid length (%d bytes)", size)
	}

	copy(recipientKey[:], b)

	// create an ephemeral key pair
	pubKey, privKey, err := generateKey(rand.Reader)
	if err != nil {
		return "", err
	}

	// create the nonce by hashing together the two public keys
	nonce := new([nonceSize]byte)
	nonceHash, err := blake2b.New(nonceSize, nil)
	if err != nil {
		return "", err
	}

	if _, err := nonceHash.Write(pubKey[:]); err != nil {
		return "", err
	}

	if _, err := nonceHash.Write(recipientKey[:]); err != nil {
		return "", err
	}

	copy(nonce[:], nonceHash.Sum(nil))

	// begin the output with the ephemeral public key and append the encrypted content
	out := box.Seal(pubKey[:], []byte(secret), nonce, recipientKey, privKey)

	// base64-encode the final output
	return base64.StdEncoding.EncodeToString(out), nil
}
