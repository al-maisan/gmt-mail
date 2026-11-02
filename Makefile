BINARY  := gmt-mail
# VERSION is derived from the latest git tag (the single source of truth for a
# release). Cut a release by creating the tag, e.g.:
#   git tag -s v0.7.0 -m "v0.7.0" && git push origin v0.7.0
# Override for a one-shot cut before the tag exists: make release VERSION=0.7.0
GIT_VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null)
VERSION     := $(patsubst v%,%,$(or $(GIT_VERSION),v0.0.0))
COMMIT  := $(shell git rev-parse --short HEAD)
DATE    := $(shell date -u +%Y-%m-%d)
LDFLAGS := -X main.appVersion=$(VERSION) -X main.gitCommit=$(COMMIT) -X main.buildDate=$(DATE)
TAG     := v$(VERSION)

# Run gh with any ambient GITHUB_TOKEN removed so it uses the keyring login;
# a narrow-scoped GITHUB_TOKEN in the environment cannot create releases.
GH := env -u GITHUB_TOKEN gh

# Per-user build dir: a fixed /tmp path could be pre-created by another user on a
# shared host to tamper with the spec/tarball before rpmbuild and upload.
BUILD_ROOT   := $(or $(XDG_CACHE_HOME),$(HOME)/.cache)/gmt-mail
RPMBUILD_DIR := $(BUILD_ROOT)/rpmbuild
TARBALL      := $(RPMBUILD_DIR)/SOURCES/gmt-mail-$(VERSION).tar.gz
SPEC_TEMPLATE := gmt-mail.spec
SPEC_OUT      := $(RPMBUILD_DIR)/SPECS/gmt-mail.spec
# gmt-mail.1 carries @VERSION@/@DATE@; the RPM spec and debian/rules fill them in from
# the package version and changelog date. `make man` renders it for local viewing.
MAN_OUT  := $(BUILD_ROOT)/man/gmt-mail.1
# Release archives + SHA256SUMS(.asc); signed with SIGNING_KEY (default: git's user.signingkey).
DIST_DIR    := $(BUILD_ROOT)/dist/$(VERSION)
SIGNING_KEY ?= $(shell git config user.signingkey)
# := ignores an ambient DIST_TARGETS, so a release always ships every target.
DIST_TARGETS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64
DIST_ASSETS  := $(foreach t,$(DIST_TARGETS),gmt-mail-$(VERSION)-$(subst /,-,$(t)).$(if $(filter windows/%,$(t)),zip,tar.gz)) \
	SHA256SUMS SHA256SUMS.asc
DIST_FILES   := $(addprefix $(DIST_DIR)/,$(DIST_ASSETS))
MAN_DATE := $(or $(shell git log -1 --format=%cs $(TAG) 2>/dev/null),$(DATE))

.PHONY: all build test vet lint fmt install clean srpm vendor tag release dist test-dist copr ppa brew test-ppa test-copr test-brew gen-spec man test-man test-pkg

all: test lint build

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test ./...

vet:
	go vet ./...

lint: vet
	golangci-lint run ./...

fmt:
	gofmt -w .

install: lint
	go install -ldflags "$(LDFLAGS)"

vendor:
	go mod vendor

tag:
	@if git rev-parse $(TAG) >/dev/null 2>&1; then \
		echo "tag: $(TAG) exists locally"; \
	else \
		git tag -s $(TAG) -m "$(TAG)"; \
		echo "tag: created $(TAG)"; \
	fi
	@if git ls-remote --tags origin $(TAG) | grep -q $(TAG); then \
		echo "tag: $(TAG) exists on remote"; \
	else \
		git push origin $(TAG); \
		echo "tag: pushed $(TAG)"; \
	fi

PREV_RELEASE := $(shell $(GH) release list --limit 1 --json tagName --jq '.[0].tagName' 2>/dev/null)

# Creates the GitHub release with the dist archives; an existing release that lacks
# any of them (none yet, or an interrupted upload) gets all of them (re)uploaded,
# and a draft left by an interrupted `gh release create` is published.
# Archives are built only when they are needed.
release: tag
	@if $(GH) release view $(TAG) >/dev/null 2>&1; then \
		have=" $$($(GH) release view $(TAG) --json assets --jq '[.assets[].name] | join(" ")') "; missing=; \
		for a in $(DIST_ASSETS); do case "$$have" in *" $$a "*) ;; *) missing="$$missing $$a";; esac; done; \
		if [ -z "$$missing" ]; then \
			echo "release: $(TAG) exists on GitHub with all assets"; \
		else \
			echo "release: $(TAG) lacks:$$missing"; \
			$(MAKE) --no-print-directory dist && $(GH) release upload --clobber $(TAG) $(DIST_FILES) \
				&& echo "release: uploaded assets to $(TAG)" || exit 1; \
		fi; \
		if [ "$$($(GH) release view $(TAG) --json isDraft --jq .isDraft)" = true ]; then \
			$(GH) release edit $(TAG) --draft=false && echo "release: published draft $(TAG)"; \
		fi; \
	elif [ -n "$(PREV_RELEASE)" ]; then \
		$(MAKE) --no-print-directory dist && \
		$(GH) release create $(TAG) --title "$(TAG)" --generate-notes --notes-start-tag $(PREV_RELEASE) $(DIST_FILES) \
			&& echo "release: created $(TAG) (diff from $(PREV_RELEASE))"; \
	else \
		$(MAKE) --no-print-directory dist && \
		$(GH) release create $(TAG) --title "$(TAG)" --generate-notes $(DIST_FILES) \
			&& echo "release: created $(TAG) (first release)"; \
	fi

# Builds the release archives from the tagged commit (a clean checkout of $(TAG)),
# dated with the tag's commit date so a rebuild gives the same archives,
# then signs SHA256SUMS (one key touch).
dist:
	@[ "$$(git rev-parse HEAD)" = "$$(git rev-parse -q --verify '$(TAG)^{commit}')" ] && [ -z "$$(git status --porcelain)" ] \
		|| { echo "dist: HEAD must be a clean checkout of $(TAG)" >&2; exit 1; }
	DIST_TARGETS="$(DIST_TARGETS)" scripts/build-dist.sh $(VERSION) $(COMMIT) $(MAN_DATE) $(DIST_DIR)
	gpg --batch --yes --armor --detach-sign $(if $(SIGNING_KEY),--local-user $(SIGNING_KEY)) $(DIST_DIR)/SHA256SUMS
	@echo "dist: signed $(DIST_DIR)/SHA256SUMS.asc"

test-dist:
	scripts/build-dist_test.sh

srpm: vendor gen-spec
	mkdir -p $(RPMBUILD_DIR)/{SOURCES,SPECS,SRPMS}
	tar czf $(TARBALL) --transform 's,^\.,gmt-mail-$(VERSION),' \
		--exclude='./.git' --exclude='./gmt-mail.spec' \
		--exclude='./ai' --exclude='./.claude' \
		--exclude='./$(BINARY)' --exclude='./ppa' .
	rpmbuild -bs $(SPEC_OUT) --define "_topdir $(RPMBUILD_DIR)"
	@echo "SRPM: $$(ls $(RPMBUILD_DIR)/SRPMS/gmt-mail-$(VERSION)-*.src.rpm)"

# Generate the final spec into the build dir from the tracked template,
# substituting @VERSION@ and splicing in the git-derived %changelog. The tracked
# gmt-mail.spec is never modified, so it can never drift or dirty the tree.
gen-spec:
	@mkdir -p $(RPMBUILD_DIR)/SPECS
	@./scripts/gen-rpm-changelog.sh > $(RPMBUILD_DIR)/rpm-changelog.tmp
	@awk -v ver='$(VERSION)' -v clog=$(RPMBUILD_DIR)/rpm-changelog.tmp \
		'{ gsub(/@VERSION@/, ver) } /^@CHANGELOG@$$/ { while ((getline l < clog) > 0) print l; next } { print }' \
		$(SPEC_TEMPLATE) > $(SPEC_OUT)
	@rm -f $(RPMBUILD_DIR)/rpm-changelog.tmp
	@echo "spec: generated $(SPEC_OUT) (version $(VERSION))"

man:
	@mkdir -p $(dir $(MAN_OUT))
	@sed -e 's/@VERSION@/$(VERSION)/g' -e 's/@DATE@/$(MAN_DATE)/g' gmt-mail.1 > $(MAN_OUT)
	@echo "man: rendered $(MAN_OUT) (version $(VERSION), $(MAN_DATE)); view with: man -l $(MAN_OUT)"

test-man: man gen-spec
	scripts/man_test.sh $(MAN_OUT) $(SPEC_OUT) $(VERSION)

# Builds for every released, still-maintained Fedora that doesn't have this version
# yet (builds the SRPM only then). COPR_FORCE=1 rebuilds; COPR_CHROOTS="..." overrides.
copr:
	scripts/copr-build.sh $(VERSION) $(RPMBUILD_DIR)/SRPMS

test-copr:
	scripts/copr-build_test.sh
	scripts/gen-rpm-changelog_test.sh

# Uploads only releases that don't have this version yet; force a rebuild of an
# already-uploaded version with: make ppa PPA_REV=2
ppa: tag vendor
	ppa/build-ppa.sh $(PPA_REV)

test-ppa:
	ppa/build-ppa_test.sh

# Bumps the Homebrew formula in al-maisan/homebrew-tap to this version (no-op if
# it is already there); needs the tag on GitHub.
brew:
	scripts/brew-update.sh $(VERSION)

test-brew:
	scripts/brew-update_test.sh

# Tests of the packaging and release scripts; CI runs them.
test-pkg: test-dist test-ppa test-copr test-brew test-man

publish: release copr ppa brew
	@echo "Published $(TAG) to GitHub, COPR, PPA and Homebrew"

clean:
	rm -f $(BINARY)
	rm -rf $(BUILD_ROOT)
