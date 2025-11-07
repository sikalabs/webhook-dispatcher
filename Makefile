default: build

-include Makefile.local.mk

build:
	go build

release:
	go mod tidy
	slu go-code version-bump --auto --tag
	goreleaser
	slu go-code version-bump --auto
	git push
