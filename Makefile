.PHONY: test test-engine test-engine-integration test-grader test-frontend test-e2e help

help:
	@echo "Frameval test targets"
	@echo ""
	@echo "  test                      Run unit tests across engine + grader + frontend"
	@echo "                            (does NOT include integration or E2E — invoke those explicitly)"
	@echo "  test-engine               Go unit tests with -race"
	@echo "  test-engine-integration   Go integration tests (build tag: integration; brings up FakeGrader, no Docker required)"
	@echo "  test-grader               Python grader tests via pytest"
	@echo "  test-frontend             Frontend unit tests via Vitest"
	@echo "  test-e2e                  Playwright end-to-end tests (requires the dev stack running)"

test: test-engine test-grader test-frontend

test-engine:
	cd engine && go test -race ./...

test-engine-integration:
	cd engine && go test -race -tags=integration ./test/integration/...

test-grader:
	cd grader && uv run pytest

test-frontend:
	cd frontend && npm test -- --run

test-e2e:
	cd frontend && npx playwright test
