.PHONY: test test-engine test-engine-integration test-grader test-frontend test-e2e help \
        ci-local ci-engine ci-grader ci-frontend lint build

help:
	@echo "Frameval test + CI targets"
	@echo ""
	@echo "  test                      Run unit tests across engine + grader + frontend"
	@echo "                            (does NOT include integration or E2E — invoke those explicitly)"
	@echo "  test-engine               Go unit tests with -race"
	@echo "  test-engine-integration   Go integration tests (build tag: integration; brings up FakeGrader, no Docker required)"
	@echo "  test-grader               Python grader tests via pytest"
	@echo "  test-frontend             Frontend unit tests via Vitest"
	@echo "  test-e2e                  Playwright end-to-end tests (requires the dev stack running)"
	@echo ""
	@echo "  lint                      Run linters across all three services"
	@echo "  build                     Build all three services (mirror of CI build steps)"
	@echo ""
	@echo "  ci-local                  Run the full CI pull_request event locally via act"
	@echo "  ci-engine | ci-grader | ci-frontend   Run a single CI job via act"

test: test-engine test-grader test-frontend

test-engine:
	cd engine && go test -race ./...

test-engine-integration:
	cd engine && go test -race -tags=integration ./test/integration/...

test-grader:
	cd grader && uv run pytest

test-frontend:
	cd frontend && npm test

test-e2e:
	cd frontend && npx playwright test

lint:
	cd engine && go vet ./...
	cd grader && uv run ruff check . || true
	cd frontend && npm run lint

build:
	cd engine && go build ./...
	cd grader && uv sync
	cd frontend && npm run build

ci-local:
	act pull_request --rm

ci-engine:
	act pull_request --rm -j engine

ci-grader:
	act pull_request --rm -j grader

ci-frontend:
	act pull_request --rm -j frontend
