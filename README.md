# Launch Control
A tool to help you bootstrap projects
## Installation
```sh
curl https://storage.googleapis.com/launchctl/install.sh | sh -
```
## Usage
### Create a GitHub PAT Token
Requires repo and workflow permissions
https://github.com/settings/tokens
![](docs/pat.jpg)
### Create a Denoy Deploy Token
https://dash.deno.com/account#access-tokens
### Export Variables
```sh
export GITHUB_TOKEN=<token>
export DENO_DEPLOY_TOKEN=<token>
```
### Run
```sh
launchctl <flavour> <project_name>
e.g. launchctl svelte-astro-deno hello-world
```
## Flavours
### [svelte-astro-deno](https://github.com/fraserdarwent/create-svelte-astro-deno-app)
Astro + Svelte on Deno Deploy via GitHub Actions
