BIN     := bin/stopan
PREFIX  ?= $(HOME)/.local
GO      ?= go
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.DEFAULT_GOAL := help

DIST_PLATFORMS ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
DEB_ARCHES ?= amd64 arm64
DEB_IMAGE ?= debian:latest
DEB_MAINTAINER ?= $(shell git config user.name) <$(shell git config user.email)>

.PHONY: help build test dist deb lint fmt check run clean install uninstall tidy

help: ## list targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  %-10s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## build bin/stopan, stamped with the git version
	$(GO) build -ldflags "-X main.version=$(VERSION)" -o $(BIN) .

test: ## run all tests (against a fake API server; no network)
	$(GO) test ./...

dist: ## build release tarballs and SHA256SUMS into bin/dist
	@set -e; rm -rf bin/dist; mkdir -p bin/dist/.completions; \
	$(GO) run . completion bash > bin/dist/.completions/stopan.bash; \
	$(GO) run . completion zsh > bin/dist/.completions/_stopan; \
	$(GO) run . completion fish > bin/dist/.completions/stopan.fish; \
	chmod 0644 bin/dist/.completions/*; \
	case "$$(tar --version)" in \
		*bsdtar*) own="--uid 0 --gid 0 --uname root --gname root";; \
		*) own="--owner=0 --group=0 --numeric-owner";; \
	esac; \
	for p in $(DIST_PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; name=stopan-cli-$(VERSION)-$$os-$$arch; \
		mkdir bin/dist/$$name; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 $(GO) build -trimpath \
			-ldflags "-s -w -X main.version=$(VERSION)" -o bin/dist/$$name/stopan .; \
		cp LICENSE README.md bin/dist/$$name/; \
		mkdir bin/dist/$$name/completions; cp bin/dist/.completions/* bin/dist/$$name/completions/; \
		COPYFILE_DISABLE=1 tar $$own -czf bin/dist/$$name.tar.gz -C bin/dist $$name; \
		rm -rf bin/dist/$$name; \
	done; \
	rm -rf bin/dist/.completions; \
	cd bin/dist; \
	if command -v sha256sum >/dev/null; then sha256sum *.tar.gz; else shasum -a 256 *.tar.gz; fi > SHA256SUMS; \
	cat SHA256SUMS

deb: ## package the linux tarballs in bin/dist as .deb files (needs docker; run dist first)
	@command -v docker >/dev/null || { echo "deb needs docker for dpkg-deb"; exit 1; }
	@set -e; ver=$$(echo $(VERSION) | sed 's/^v//; s/-/+/; s/-/./g'); \
	for arch in $(DEB_ARCHES); do \
		tgz=bin/dist/stopan-cli-$(VERSION)-linux-$$arch.tar.gz; \
		[ -f $$tgz ] || { echo "deb: $$tgz missing; run make dist"; exit 1; }; \
		root=bin/dist/deb-$$arch; rm -rf $$root; \
		mkdir -p $$root/DEBIAN $$root/usr/bin $$root/usr/share/doc/stopan-cli $$root/usr/share/lintian/overrides; \
		echo "stopan-cli: statically-linked-binary [usr/bin/stopan]" > $$root/usr/share/lintian/overrides/stopan-cli; \
		tar -xzf $$tgz -O stopan-cli-$(VERSION)-linux-$$arch/stopan > $$root/usr/bin/stopan; \
		chmod 0755 $$root/usr/bin/stopan; \
		comp=stopan-cli-$(VERSION)-linux-$$arch/completions; \
		mkdir -p $$root/usr/share/bash-completion/completions $$root/usr/share/zsh/vendor-completions $$root/usr/share/fish/vendor_completions.d; \
		tar -xzf $$tgz -O $$comp/stopan.bash > $$root/usr/share/bash-completion/completions/stopan; \
		tar -xzf $$tgz -O $$comp/_stopan > $$root/usr/share/zsh/vendor-completions/_stopan; \
		tar -xzf $$tgz -O $$comp/stopan.fish > $$root/usr/share/fish/vendor_completions.d/stopan.fish; \
		chmod 0644 $$root/usr/share/bash-completion/completions/stopan $$root/usr/share/zsh/vendor-completions/_stopan $$root/usr/share/fish/vendor_completions.d/stopan.fish; \
		{ echo "Format: https://www.debian.org/doc/packaging-manuals/copyright-format/1.0/"; \
		  echo "Upstream-Name: stopan-cli"; echo "Source: https://github.com/t0d0r/stopan-cli"; echo; \
		  echo "Files: *"; echo "License: BSD-2-Clause"; echo; \
		  sed 's/^$$/./; s/^/ /' LICENSE; } > $$root/usr/share/doc/stopan-cli/copyright; \
		printf 'stopan-cli (%s) unstable; urgency=medium\n\n  * Release %s, see https://github.com/t0d0r/stopan-cli/releases\n\n -- %s  %s\n' \
			"$$ver" "$(VERSION)" "$(DEB_MAINTAINER)" "$$(LC_ALL=C date -R 2>/dev/null || LC_ALL=C date '+%a, %d %b %Y %H:%M:%S %z')" \
			| gzip -9n > $$root/usr/share/doc/stopan-cli/changelog.gz; \
		chmod 0644 $$root/usr/share/doc/stopan-cli/* $$root/usr/share/lintian/overrides/stopan-cli; \
		{ echo "Package: stopan-cli"; echo "Version: $$ver"; echo "Architecture: $$arch"; \
		  echo "Maintainer: $(DEB_MAINTAINER)"; \
		  echo "Installed-Size: $$(du -sk $$root/usr | cut -f1)"; \
		  echo "Recommends: ca-certificates"; echo "Suggests: xdg-utils"; \
		  echo "Section: utils"; echo "Priority: optional"; \
		  echo "Homepage: https://github.com/t0d0r/stopan-cli"; \
		  echo "Description: command line and terminal UI for the Stopan service desk"; \
		  echo " stopan lists, reads, answers and changes tickets of a Stopan service"; \
		  echo " desk through its JSON API with a personal token, from the shell or in"; \
		  echo " a full-screen terminal dashboard."; \
		  echo " The binary is static and has no library dependencies."; } > $$root/DEBIAN/control; \
		docker run --rm --user $$(id -u):$$(id -g) -v $(CURDIR)/bin/dist:/dist $(DEB_IMAGE) \
			dpkg-deb --root-owner-group -Zxz --build /dist/deb-$$arch /dist/stopan-cli_$${ver}_$$arch.deb; \
		rm -rf $$root; \
	done; \
	cd bin/dist; \
	if command -v sha256sum >/dev/null; then sha256sum *.tar.gz *.deb; else shasum -a 256 *.tar.gz *.deb; fi > SHA256SUMS; \
	cat SHA256SUMS

lint: ## check gofmt and run go vet
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	$(GO) vet ./...

fmt: ## format all Go files
	gofmt -w .

check: lint test ## lint and test; must pass before a commit

run: build ## build and start the dashboard (needs stopan login first)
	./$(BIN)

install: build ## copy the binary to $(PREFIX)/bin
	install -d $(PREFIX)/bin
	install -m 0755 $(BIN) $(PREFIX)/bin/stopan

uninstall: ## remove the binary from $(PREFIX)/bin
	rm -f $(PREFIX)/bin/stopan

tidy: ## tidy go.mod from the local module cache only
	GOPROXY=off $(GO) mod tidy -e

clean: ## remove build output
	rm -rf bin
