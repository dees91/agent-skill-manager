BIN ?= $(HOME)/.local/bin/skill-manager
WAILS_VERSION ?= v2.13.0
WAILS = go run github.com/wailsapp/wails/v2/cmd/wails@$(WAILS_VERSION)

.PHONY: dev install build test test-all gui-dev gui-bindings gui-test gui-build notices notices-check release-package clean

dev: install

install:
	@mkdir -p "$(dir $(BIN))"
	go build -o "$(BIN)" .
	@echo "Installed $(BIN)"

build:
	@mkdir -p bin
	go build -o bin/skill-manager .

test:
	go test ./...

test-all: test gui-test

gui-dev:
	cd desktop && $(WAILS) dev

gui-bindings:
	cd desktop && $(WAILS) generate module

gui-test:
	cd desktop/frontend && npm ci
	cd desktop/frontend && npm run typecheck
	cd desktop/frontend && npm test
	cd desktop/frontend && npm run build
	cd desktop && go test ./...

gui-build:
	cd desktop && $(WAILS) build -platform darwin/arm64 -clean -nocolour -skipbindings
	@echo "Built desktop/build/bin/Skill Manager.app (local ad-hoc signed darwin/arm64)"

notices:
	@scripts/generate-third-party-notices.sh > THIRD_PARTY_NOTICES.txt

notices-check:
	@temporary="$$(mktemp)"; \
	trap 'rm -f "$$temporary"' EXIT; \
	scripts/generate-third-party-notices.sh > "$$temporary"; \
	diff -u THIRD_PARTY_NOTICES.txt "$$temporary"

release-package:
	@RELEASE_VERSION="$(RELEASE_VERSION)" ./scripts/package-release.sh

clean:
	rm -rf bin
