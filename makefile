# ============================================================
# Root Makefile — Autonomous-CI-Platform
# Orchestrator (Go) is run natively; ai-service/Postgres/Redis
# run via docker-compose. See docker-compose.yml for details.
# ============================================================

COMPOSE      = docker-compose
ORCH_DIR     = orchestrator
BUILD_DIR    = $(ORCH_DIR)/out
BINARY_NAME  = orchestrator-bin

.PHONY: help dev dev-up dev-down dev-restart dev-logs migrate-reset \
        orch-run orch-build orch-build-linux orch-build-windows orch-clean \
        run-windows test test-orch test-ai fmt

help:
	@echo "Infra (ai-service, postgres, redis):"
	@echo "  make dev-up          # build + start infra in the background"
	@echo "  make dev-down        # stop infra"
	@echo "  make dev-restart     # dev-down + dev-up"
	@echo "  make dev-logs        # tail ai-service + celery-worker logs"
	@echo "  make migrate-reset   # wipe DB volume (re-runs migrations on next dev-up)"
	@echo ""
	@echo "Orchestrator (native):"
	@echo "  make orch-run        # go run ./cmd (run this in its own terminal)"
	@echo "  make orch-build      # build binary for current OS/arch"
	@echo "  make orch-build-linux / orch-build-windows"
	@echo "  make orch-clean      # remove build artifacts"
	@echo "  make run-windows     # build for windows, start ngrok, run binary"
	@echo ""
	@echo "Combined:"
	@echo "  make dev             # start infra, then reminds you to run orch-run"
	@echo "  make test            # go test ./... + pytest inside ai-service container"
	@echo "  make fmt             # gofmt the orchestrator"

# ---------------------------------------------------------------
# Infra: ai-service, postgres, redis (docker-compose)
# ---------------------------------------------------------------

dev-up:
	$(COMPOSE) up --build -d

dev-down:
	$(COMPOSE) down

dev-restart: dev-down dev-up

dev-logs:
	$(COMPOSE) logs -f ai-service celery-worker

# Drops the postgres volume so migrations/*.sql re-run on next dev-up.
migrate-reset:
	$(COMPOSE) down -v

# ---------------------------------------------------------------
# Orchestrator (native — NOT containerized, see architecture notes)
# ---------------------------------------------------------------

orch-run:
	cd $(ORCH_DIR) && go run ./cmd

orch-build:
	cd $(ORCH_DIR) && go build -o out/$(BINARY_NAME) ./cmd

orch-build-linux:
	cd $(ORCH_DIR) && GOOS=linux GOARCH=amd64 go build -o out/$(BINARY_NAME)-linux-amd64 ./cmd

orch-build-windows:
	cd $(ORCH_DIR) && GOOS=windows GOARCH=amd64 go build -o out/$(BINARY_NAME).exe ./cmd

orch-clean:
	cd $(ORCH_DIR) && go clean && rm -rf out

fmt:
	cd $(ORCH_DIR) && gofmt -l -w .

# Matches the old orchestrator/makefile's run-windows target:
# build for windows, launch ngrok, run the binary.
run-windows: orch-clean orch-build-windows
	@echo Starting ngrok and app...
	powershell -Command "Start-Process ngrok -ArgumentList 'http 8080'"
	$(ORCH_DIR)/out/$(BINARY_NAME).exe

# ---------------------------------------------------------------
# Combined
# ---------------------------------------------------------------

dev: dev-up
	@echo "ai-service, postgres, redis are up (docker-compose)."
	@echo "Now run 'make orch-run' in a separate terminal to start the orchestrator."

test-orch:
	cd $(ORCH_DIR) && go test ./...

test-ai:
	$(COMPOSE) run --rm ai-service pytest

test: test-orch test-ai