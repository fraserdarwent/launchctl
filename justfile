set dotenv-load
test:
	go test
run:
	go run launchctl.go svelte-cloudflare lctl-run
build:
	go build
release: build
	gsutil cp install.sh gs://launchctl
	gsutil cp launchctl gs://launchctl
