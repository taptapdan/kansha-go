#!/usr/bin/env just --justfile

# list recipes
default:
    @just --list

# rebuild web/dist/kansha.css from the templates (needs Node for npx)
css:
    npx -y tailwindcss@3.4.17 -c web/tailwind.config.js -i web/css/input.css -o web/dist/kansha.css --minify

# run the tests
test:
    go test ./...

# preview the sample module's docs with live reload
demo:
    go run ./cmd/kansha serve -C testdata/shop

# preview kansha's own docs
docs:
    go run ./cmd/kansha serve
