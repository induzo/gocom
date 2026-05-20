.PHONY: all clean docs-gen \
		changelogs-gen \
		readme-version-table-update \
		lint sec-scan upgrade release release-tag push-tag test-all coverage test leak \
		bench bench-compare

help: ## show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf "\033[36m%-25s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

all: docs-gen changelogs-gen readme-version-table-update

ALL_MODULES=$(shell go work edit -json | sed -n 's/.*"DiskPath": "\(.*\)".*/\1/p' | sed 's|^\./||')

ALL_MODULES_SPACE_SEP=$(shell echo $(ALL_MODULES) | xargs printf "%s ")

ALL_MODULES_DOTDOTDOT=$(shell echo $(ALL_MODULES) | xargs printf "./%s/... ")

SHELL = /bin/bash

########
# docs #
########

test-echo:
	echo $(ALL_MODULES_SPACE_SEP)

docs-gen: ## generate docs for every module, as markdown thanks to https://github.com/princjef/gomarkdoc
	@( \
		for module in $(ALL_MODULES_SPACE_SEP); do \
			gomarkdoc --output ./$$module/README.md ./$$module/ && \
			printf "docs generated for $$module!\n"; \
			git commit -m "docs: update docs for module $$module" ./$$module/README.md; \
		done \
	)


##############
# changelogs #
##############

changelogs-gen: ## generate changelog for every module.
	@for module in $(ALL_MODULES_SPACE_SEP); do \
		awk -v module="$$module" '{gsub(/TAG_MODULE/, module); print}' ./cliff.toml > ./cliff.toml.tmp && \
		mv ./cliff.toml.tmp ./cliff.toml && \
		git cliff \
			--include-path "**/$$module/*" \
			-o ./$$module/CHANGELOG.md && \
		awk -v module="$$module" '{gsub(module, "TAG_MODULE"); print}' ./cliff.toml > ./cliff.toml.tmp && \
		mv ./cliff.toml.tmp ./cliff.toml && \
		printf "\nchangelog generated for $$module!\n"; \
		git commit -m "docs(changelog): update CHANGELOG.md for $$(git describe --abbrev=0 --tags $$(git rev-list --tags="$$module/v[0-9].*" --max-count=1))" ./$$module/CHANGELOG.md; \
	done



#############################
# versions update in README #
#############################

readme-version-table-update: ## update version table in README.md to latest version.
	@for module in $(ALL_MODULES_SPACE_SEP); do \
		latest=$$(git tag --list "$$module/v*" | sort -V | tail -n1 | sed "s|$$module/v||"); \
		[ -z "$$latest" ] && continue; \
		awk -F'|' -v OFS='|' -v module="$$module" -v latest="$$latest" \
			'index($$0, "]("module")") { sub(/[0-9]+\.[0-9]+\.[0-9]+/, latest, $$4) } 1' \
			README.md > README.md.tmp && mv README.md.tmp README.md; \
	done; \
	git commit -m "docs(readme): update latest versions" ./README.md


########
# lint #
########

lint-all: ## lints the entire codebase
	@( \
		for module in $(ALL_MODULES_SPACE_SEP); do \
			if [ -z $$module ]; then \
				break; \
			fi; \
			pushd $$module > /dev/null; \
			echo "checking" $$module; \
			golangci-lint run ; \
			popd > /dev/null; \
		done \
	)

lint-specific: ## lints a specific module
	@( \
		select module in $(ALL_MODULES_SPACE_SEP); do \
			if [ -z $$module ]; then \
				break; \
			fi; \
			pushd $$module > /dev/null; \
			golangci-lint run ; \
			popd > /dev/null; \
			break; \
		done \
	)

#######
# sec #
#######

sec-scan: trivy-scan vuln-scan-all ## scan for sec issues

trivy-scan: ## scan for sec issues with trivy (trivy binary needed)
	trivy fs --exit-code 1 --no-progress --severity HIGH ./

vuln-scan-all: ## scan for sec issues with govulncheck (govulncheck binary needed)
	@( \
		for module in $(ALL_MODULES_SPACE_SEP); do \
			if [ -z $$module ]; then \
				break; \
			fi; \
			pushd $$module > /dev/null && \
			govulncheck ./...  && \
			popd > /dev/null; \
		done \
	)

###########
# upgrade #
###########

upgrade: ## upgrade selection module dependencies (beware, it can break everything)
	@( \
		select module in $(ALL_MODULES_SPACE_SEP); do \
			if [ -z $$module ]; then \
				break; \
			fi; \
			pushd $$module > /dev/null; \
			go mod tidy && \
			go get -t -u ./... && \
			go mod tidy ; \
			popd > /dev/null; \
			break; \
		done \
	)

########
# deps #
########

download-all: ## download all dependencies for the different modules
	@( \
		for module in $(ALL_MODULES_SPACE_SEP); do \
			if [ -z $$module ]; then \
				break; \
			fi; \
			pushd $$module > /dev/null; \
			go mod download ; \
			popd > /dev/null; \
		done \
	)

###########
# release #
###########

release: release-specific readme-version-table-update ## release selection module, gen-changelog, gen docs, commit and tag and update the version table

release-tag: ## create an annotated module tag. Usage: make release-tag MODULE=http/health VERSION=1.2.0
	@[[ -n "$(MODULE)" ]] || (echo "MODULE is required, e.g. MODULE=http/health" && exit 1)
	@[[ -n "$(VERSION)" ]] || (echo "VERSION is required, e.g. VERSION=1.2.0" && exit 1)
	@[[ -d "./$(MODULE)" ]] || (echo "module path does not exist: ./$(MODULE)" && exit 1)
	@[[ "$(VERSION)" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.-]+)?$$ ]] || (echo "invalid VERSION: $(VERSION), expected X.Y.Z" && exit 1)
	@TAG="$(MODULE)/v$(VERSION)"; \
	if git rev-parse -q --verify "refs/tags/$$TAG" >/dev/null; then \
		echo "tag already exists: $$TAG"; \
		exit 1; \
	fi; \
	git tag -a "$$TAG" -m "release: $(MODULE) v$(VERSION)"; \
	echo "created tag $$TAG"

push-tag: ## push a module tag. Usage: make push-tag MODULE=http/health VERSION=1.2.0
	@[[ -n "$(MODULE)" ]] || (echo "MODULE is required, e.g. MODULE=http/health" && exit 1)
	@[[ -n "$(VERSION)" ]] || (echo "VERSION is required, e.g. VERSION=1.2.0" && exit 1)
	@TAG="$(MODULE)/v$(VERSION)"; \
	git rev-parse -q --verify "refs/tags/$$TAG" >/dev/null || (echo "local tag not found: $$TAG" && exit 1); \
	git push origin "$$TAG"

release-specific: ## release selection module, gen-changelog, gen docs, commit and tag
	@select module in $(ALL_MODULES_SPACE_SEP); do \
		[ -z "$$module" ] && break; \
		printf "latest tag for $$module: "; \
		git tag --list --sort=version:refname "$$module/v*" | tail -1; \
		printf "new tag (form $$module/vX.Y.Z): "; \
		read -r TAG; \
		sed -i.bak -E "s:TAG_MODULE:$$module:g" ./cliff.toml && \
		git cliff --tag $$TAG --include-path "**/$$module/*" -o ./$$module/CHANGELOG.md && \
		mv ./cliff.toml.bak ./cliff.toml && \
		{ git diff --quiet -- ./$$module/CHANGELOG.md || git commit -m "docs(changelog): update CHANGELOG.md for $$TAG" ./$$module/CHANGELOG.md; } && \
		gomarkdoc --output ./$$module/README.md ./$$module/ && \
		{ git diff --quiet -- ./$$module/README.md || git commit -m "docs: update docs for module $$module" ./$$module/README.md; } && \
		go work sync && \
		{ git diff --quiet -- ./go.work ./go.work.sum || git commit -m "chore: update go.work" ./go.work ./go.work.sum; } && \
		git tag $$TAG && \
		printf "\nrelease tagged $$TAG\nif everything looks good, run: git push origin $$TAG\n"; \
		break; \
	done


#########
# tests #
#########

test-all:
	@go test -race -failfast $(ALL_MODULES_DOTDOTDOT)

coverage:
	go test $(ALL_MODULES_DOTDOTDOT) -race -failfast -covermode=atomic -coverprofile=./coverage.out

test: ## launch tests for a selection module
	@( \
		select module in $(ALL_MODULES_SPACE_SEP); do \
			if [ -z $$module ]; then \
				break; \
			fi; \
			go test ./$$module/... -cover -race -covermode=atomic -failfast -coverprofile=./$$module/coverage.out; \
			break; \
		done \
	)

bench: ## launch bench for a selection module
	@( \
		select module in $(ALL_MODULES_SPACE_SEP); do \
			if [ -z $$module ]; then \
				break; \
			fi; \
			go test ./$$module/... -bench=. -benchmem | tee ./$$module/bench.txt; \
			break; \
		done \
	)

bench-compare: ## compare benchs results for selection module
	@( \
		select module in $(ALL_MODULES_SPACE_SEP); do \
			if [ -z $$module ]; then \
				break; \
			fi; \
			benchstat ./$$module/bench.txt; \
			break; \
		done \
	)

###########
#   GCI   #
###########

gci-format: ## format repo through gci linter
	gci write ./ --skip-generated -s standard -s default -s "Prefix(github.com/triple-a)"
