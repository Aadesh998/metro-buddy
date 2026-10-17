.PHONY: install build build-server build-admin build-mobile dev-server dev-admin dev-mobile

install:
	cd admin && npm ci
	cd apps/mobile && npm ci
	cd server && go mod download

build: build-server build-admin build-mobile

build-server:
	cd server && go build -o bin/server ./cmd/server

build-admin:
	cd admin && npm run build

build-mobile:
	cd apps/mobile && npm run build

dev-server:
	cd server && go run ./cmd/server

dev-admin:
	cd admin && npm run dev

dev-mobile:
	cd apps/mobile && npx expo start
