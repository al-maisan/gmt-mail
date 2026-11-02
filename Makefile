BINARY  := gmt-mail
# VERSION is derived from the latest git tag (the single source of truth for a
# release). Cut a release by creating the tag, e.g.:
#   git tag -s v0.7.0 -m "v0.7.0" && git push origin v0.7.0
# Override for a one-shot cut before the tag exists: make release VERSION=0.7.0
GIT_VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null)
VERSION     := $(patsubst v%,%,$(or $(GIT_VERSION),v0.0.0))
# A fixed length: the default grows with the clone's object count, and the
# commit is in the binaries.
COMMIT  := $(shell git rev-parse --short=7 HEAD)
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
# Official Go toolchain of the release builds, so anyone can rebuild the same
# archives. Bump it with Go patch releases.
RELEASE_GO ?= go1.26.10
# A stable version (X.Y.Z) also gets .deb packages and the container image.
STABLE := $(shell echo '$(VERSION)' | grep -xE '[0-9]+\.[0-9]+\.[0-9]+')
# Container image of the archives in DIST_DIR, as an OCI layout.
IMAGE_DIR := $(DIST_DIR)/image
# Checks the tag (scripts/release-check.sh) and, for a stable version, the
# ghcr.io login; runs lint and test, then builds and signs the archives and, for
# a stable version, builds the image from them.
RELEASE_DIST = scripts/release-check.sh $(TAG) \
	$(if $(STABLE),&& bash -c 'source scripts/release-lib.sh && ghcr_login >/dev/null') \
	&& $(MAKE) --no-print-directory lint test dist $(if $(STABLE),image)
# After the GitHub release: a stable version's image goes to ghcr.io.
IMAGE := ghcr.io/al-maisan/gmt-mail
RELEASE_PUSH = $(if $(STABLE),&& $(MAKE) --no-print-directory push-image)
# Prereleases (vX.Y.Z-PRE) are marked as such on GitHub.
RELEASE_FLAGS = --verify-tag $(if $(findstring -,$(TAG)),--prerelease) \
	--notes "Built with $(RELEASE_GO). Verify a download: https://github.com/al-maisan/gmt-mail/blob/master/docs/install.md\#verify-a-download"
# := ignores an ambient DIST_TARGETS, so a release always ships every target.
DIST_TARGETS := linux/amd64 linux/arm64 linux/armv6 darwin/amd64 darwin/arm64 windows/amd64
# A stable version also gets a .deb per linux target (armv6 is Debian's armhf).
DEB_ARCHS    := $(subst armv6,armhf,$(patsubst linux/%,%,$(filter linux/%,$(DIST_TARGETS))))
DIST_DEBS    := $(if $(STABLE),$(foreach a,$(DEB_ARCHS),gmt-mail_$(VERSION)-1_$(a).deb))
DIST_ASSETS  := $(foreach t,$(DIST_TARGETS),gmt-mail-$(VERSION)-$(subst /,-,$(t)).$(if $(filter windows/%,$(t)),zip,tar.gz)) \
	$(DIST_DEBS) SHA256SUMS SHA256SUMS.asc
DIST_FILES   := $(addprefix $(DIST_DIR)/,$(DIST_ASSETS))
MAN_DATE := $(or $(shell git log -1 --format=%cs $(TAG) 2>/dev/null),$(DATE))

.PHONY: all build test vet lint fmt install clean srpm vendor tag release dist test-dist copr ppa brew test-ppa test-copr test-brew gen-spec man test-man doc-version test-release test-pkg image push-image test-image

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

PREV_RELEASE := $(shell $(GH) release list --exclude-drafts --exclude-pre-releases --limit 1 --json tagName --jq '.[0].tagName' 2>/dev/null)

# Creates the GitHub release with the dist archives; an existing release that lacks
# any of them (none yet, or an interrupted upload) gets all of them (re)uploaded,
# and a draft left by an interrupted `gh release create` is published. A stable
# release whose image is not on ghcr.io (e.g. a failed push) gets it pushed.
# Archives are built only when they are needed.
release: tag
	@if $(GH) release view $(TAG) >/dev/null 2>&1; then \
		have=" $$($(GH) release view $(TAG) --json assets --jq '[.assets[].name] | join(" ")') "; missing=; \
		for a in $(DIST_ASSETS); do case "$$have" in *" $$a "*) ;; *) missing="$$missing $$a";; esac; done; \
		if [ -z "$$missing" ]; then \
			echo "release: $(TAG) exists on GitHub with all assets"; \
			$(if $(STABLE),if ! skopeo inspect --raw docker://$(IMAGE):$(TAG) >/dev/null 2>&1; then \
				echo "release: $(IMAGE):$(TAG) not found (an earlier run was interrupted?)"; \
				$(MAKE) --no-print-directory push-image || exit 1; \
			fi;) \
		else \
			echo "release: $(TAG) lacks:$$missing"; \
			$(RELEASE_DIST) && $(GH) release upload --clobber $(TAG) $(DIST_FILES) \
				&& echo "release: uploaded assets to $(TAG)" $(RELEASE_PUSH) || exit 1; \
		fi; \
		if [ "$$($(GH) release view $(TAG) --json isDraft --jq .isDraft)" = true ]; then \
			$(GH) release edit $(TAG) --draft=false && echo "release: published draft $(TAG)"; \
		fi; \
	elif [ -n "$(PREV_RELEASE)" ]; then \
		$(RELEASE_DIST) && \
		$(GH) release create $(TAG) --title "$(TAG)" --generate-notes --notes-start-tag $(PREV_RELEASE) $(RELEASE_FLAGS) $(DIST_FILES) \
			&& echo "release: created $(TAG) (diff from $(PREV_RELEASE))" $(RELEASE_PUSH); \
	else \
		$(RELEASE_DIST) && \
		$(GH) release create $(TAG) --title "$(TAG)" --generate-notes $(RELEASE_FLAGS) $(DIST_FILES) \
			&& echo "release: created $(TAG) (first release)" $(RELEASE_PUSH); \
	fi

# Builds the release archives from the tagged commit (a clean checkout of $(TAG)),
# dated with the tag's commit date so a rebuild gives the same archives,
# with the pinned Go toolchain, then signs SHA256SUMS (one key touch) and checks
# that the release key made the signature.
dist:
	@[ "$$(git rev-parse HEAD)" = "$$(git rev-parse -q --verify '$(TAG)^{commit}')" ] && [ -z "$$(git status --porcelain)" ] \
		|| { echo "dist: HEAD must be a clean checkout of $(TAG)" >&2; exit 1; }
	GOTOOLCHAIN=$(RELEASE_GO) DIST_TARGETS="$(DIST_TARGETS)" scripts/build-dist.sh $(VERSION) $(COMMIT) $(MAN_DATE) $(DIST_DIR)
	gpg --batch --yes --armor --detach-sign $(if $(SIGNING_KEY),--local-user $(SIGNING_KEY)) $(DIST_DIR)/SHA256SUMS
	@bash -c 'source scripts/release-key.sh && sums_signed "$$1"' _ $(DIST_DIR)/SHA256SUMS \
		|| { echo "dist: SHA256SUMS.asc is not signed by the release key" >&2; exit 1; }
	@echo "dist: signed $(DIST_DIR)/SHA256SUMS.asc"

# Sets the version of the download example in docs/install.md: make doc-version V=1.2.3,
# before tagging v1.2.3. `make release` refuses a stale one.
doc-version:
	scripts/doc-version.sh set '$(V)'

# Multi-arch container image of the archives in DIST_DIR (make dist), as an OCI
# layout in IMAGE_DIR and as localhost/gmt-mail:$(TAG). Needs podman or docker.
image:
	scripts/image.sh $(TAG) $(DIST_DIR) $(IMAGE_DIR)

# Rebuilds the image of the stable $(TAG) from its signed GitHub release assets
# and pushes it to ghcr.io; `make release` runs it. Needs a clean checkout of $(TAG).
push-image:
	scripts/push-image.sh $(TAG) $(IMAGE_DIR) \
		|| { echo "push-image: fix the cause, then run: git checkout $(TAG) && make push-image VERSION=$(VERSION)" >&2; exit 1; }

# Builds the image from test binaries and runs it; skipped without podman or
# docker. Needs network for the CA bundle unless it is cached.
test-image:
	scripts/image_test.sh

test-dist:
	scripts/build-dist_test.sh
	scripts/deb_test.sh
	scripts/install-docs_test.sh

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
	scripts/src-tarball_test.sh

# Bumps the Homebrew formula in al-maisan/homebrew-tap to this version (no-op if
# it is already there); needs the tag on GitHub.
brew:
	scripts/brew-update.sh $(VERSION)

test-brew:
	scripts/brew-update_test.sh

test-release:
	scripts/release-check_test.sh
	scripts/doc-version_test.sh
	scripts/push-image_test.sh
	scripts/make-recipes_test.sh

# Tests of the packaging and release scripts; CI runs them.
test-pkg: test-dist test-ppa test-copr test-brew test-man test-release

# A prerelease goes to GitHub only: RPM versions can't hold '-', dpkg sorts
# 1.0.0-rc1-1ppa1 above 1.0.0, and Homebrew would move to it.
publish: release $(if $(findstring -,$(TAG)),,copr ppa brew)
	@echo "Published $(TAG) to $(if $(findstring -,$(TAG)),GitHub (prerelease),GitHub, COPR, PPA and Homebrew)"

clean:
	rm -f $(BINARY)
	rm -rf $(BUILD_ROOT)
