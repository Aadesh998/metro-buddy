.PHONY: install build build-server build-admin build-mobile check check-server check-admin check-mobile dev-server dev-admin dev-mobile db-up admin-user

install:
	cd admin && npm ci
	cd apps/mobile && npm ci
	cd server && go mod download

build: build-admin build-server build-mobile

build-server:
	cd server && go build -o bin/server ./cmd/server

build-admin:
	cd admin && npm run build
	find server/internal/admin/dist -mindepth 1 ! -name .gitkeep -delete
	cp -r admin/dist/. server/internal/admin/dist/

build-mobile:
	cd apps/mobile && npm run build

check: check-server check-admin check-mobile

check-server:
	cd server && go vet ./... && go test -race ./... && go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...

check-admin:
	cd admin && npm run typecheck && npm run lint && npm run audit

check-mobile:
	cd apps/mobile && npm run typecheck && npm run lint && npm run audit

dev-server:
	cd server && set -a && . ../.env && set +a && go run ./cmd/server

dev-admin:
	cd admin && npm run dev

dev-mobile:
	cd apps/mobile && npx expo start

db-up:
	docker compose -p metro-buddy up -d --wait

admin-user:
	cd server && set -a && . ../.env && set +a && go run ./cmd/admin -email $(EMAIL) -role $(ROLE)
