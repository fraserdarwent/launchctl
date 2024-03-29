# Launch Control
A tool to help you rapidly bootstrap web projects, taking care of:
  - Repository creation from a template
  - Serverless project set up
  - Project and repository linking for continuous deployment
## Installation
```sh
curl https://storage.googleapis.com/launchctl/install.sh | sh -
```
## Usage
### Export Relevant Variables
```sh
export GITHUB_TOKEN=<token>
export DENO_DEPLOY_TOKEN=<token> # Only required for Deno Deploy flavours
export DENO_DEPLOY_TOKEN=<token> # Only required for Cloudflare Pages flavours
export CLOUDFLARE_ACCOUNT_ID=<account_id> # Only required for Cloudflare Pages flavours
```
### Run
```sh
launchctl <flavour> <project_name> # e.g. launchctl svelte-astro-deno hello-world
```
## Flavours
### [svelte-cloudflare](https://github.com/fraserdarwent/launchctl-svelte-cloudflare)
SvelteKit on Cloudflare Pages via GitHub Actions
### [svelte-astro-deno](https://github.com/fraserdarwent/launchctl-svelte-astro-deno)
Astro + Svelte on Deno Deploy via GitHub Actions
## Token Creation Help
### Create a GitHub PAT Token
Go to your [GitHub profile](https://github.com/settings/tokens) and create a token with repo and workflow permissions
![](docs/pat.jpg)
### Create a Denoy Deploy Token
Go to your [Deno profile](https://dash.deno.com/account#access-tokens) and create an access token
### Create a Cloudflare Pages Token
Go to your [Cloudflare profile](https://dash.cloudflare.com/profile/api-tokens) and create a token with Cloudflare Pages edit permission
![](docs/cloudflare-token.png)
