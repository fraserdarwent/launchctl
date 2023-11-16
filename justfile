set dotenv-load
test:
	go test
run:
	go run launchctl.go svelte-cloudflare lctl-run
build:
	go build
release: build
 