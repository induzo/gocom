.PHONY: all clean docs-gen changelogs-gen readme-version-table-update lint sec-scan upgrade release test-all coveralls test leak bench bench-compare

help: ## show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf "\033[36m%-25s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

all: docs-gen changelogs-gen readme-version-table-update

ALL_MODULES=$(shell go work edit -json | grep ModPath | sed -E 's:^.*gocom/(.*)":\1:' | sed -E 's:/v[0-9]+$$::')

ALL_MODULES_SPACE_SEP=$(shell echo $(ALL_MODULES) | xargs printf "%s ")

ALL_MODULES_DOTDOTDOT=$(shell echo $(ALL_MODULES) | xargs printf "./%s/... ")

SHELL = /bin/bash

########
# docs #
########

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
	@( \
		while IFS= read -r line; do \
			module=$$(echo "$$line" | grep -oE '\[\w+/\w+\]' | tr -d '[]'); \
			if [ ! -z "$$module" ]; then \
				latest_version=$$(git tag --list "$${module}/v*" | sed "s:$${module}/v::" | sort -t . -k1n -k2n -k3n | tail -n 1); \
				echo "$$line" | sed "s:\(.*| \)[[:digit:]]\+\(\.[[:digit:]]\+\)\{0,2\}\(.*\):\1$$latest_version\3:"; \
			else \
				echo "$$line"; \
			fi; \
		done < README.md > README.md.tmp; \
		mv README.md.tmp README.md; \
		git commit -m "docs(readme): update latest versions" ./README.md \
	)


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

release-specific: ## release selection module, gen-changelog, gen docs, commit and tag
	@( \
		select module in $(ALL_MODULES_SPACE_SEP); do \
			if [ -z $$module ]; then \
				break; \
			fi; \
			printf "here is the $$module latest tag present: "; \
			git describe --abbrev=0 --tags $$(git rev-list --tags="$$module/v[0-9].*" --max-count=1); \
			printf "what tag do you want to give? (use the form $$module/vX.X.X): "; \
			read -r TAG; \
			sed -i '' -E "s:TAG_MODULE:$$module:g" ./cliff.toml && \
			git cliff \
				--tag $$TAG \
				--include-path "**/$$module/*" \
				-o ./$$module/CHANGELOG.md && \
			sed -i '' -E "s:$$module:TAG_MODULE:g" ./cliff.toml && \
			printf "\nchangelog generated for $$module!\n"; \
			git add ./$$module/CHANGELOG.md && \
			git commit -m "docs(changelog): update CHANGELOG.md for $$TAG" ./$$module/CHANGELOG.md; \
			gomarkdoc --output ./$$module/README.md ./$$module/ && \
			printf "docs generated for $$module!\n"; \
			git add ./$$module/README.md && \
			git commit -m "docs: update docs for module $$module" ./$$module/README.md; \
			git tag $$TAG && \
			printf "\nrelease tagged $$TAG !\n"; \
			printf "\nrelease and tagging has been done, if you are OK with everything, just git push origin $$(git describe --abbrev=0 --tags $$(git rev-list --tags="$$module/v[0-9].*" --max-count=1))\n"; \
			break; \
		done \
	)


#########
# tests #
#########

test-all: test-all-race test-all-leak ## launch tests for all modules

test-all-race:
	@go test -race -failfast $(ALL_MODULES_DOTDOTDOT)

test-all-leak:
	@( \
		for module in $(ALL_MODULES_SPACE_SEP); do \
			if [ -z $$module ]; then \
				break; \
			fi; \
			pushd $$module > /dev/null && \
			go test -leak -failfast ./...  && \
			popd > /dev/null; \
		done \
	)

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

test-race: ## launch tests for a selection module with race detection
	@( \
		select module in $(ALL_MODULES_SPACE_SEP); do \
			if [ -z $$module ]; then \
				break; \
			fi; \
			go test ./$$module/... -cover -race; \
			break; \
		done \
	)


test-leak: ## launch tests for a selection module with leak detection (if enabled)
	@( \
		select module in $(ALL_MODULES_SPACE_SEP); do \
			if [ -z $$module ]; then \
				break; \
			fi; \
			go test ./$$module/... -leak; \
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
	gci write ./ --skip-generated -s standard -s default -s "Prefix(github.com/induzo)"
