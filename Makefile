.PHONY: build web-install web-dev web-build test lint vet run clean

UPX := $(shell command -v upx 2>/dev/null)

# 完整构建：前端 build → 拷入 embed → go build
build:
	cd web && pnpm build
	mkdir -p internal/server/dist
	find internal/server/dist -mindepth 1 ! -name placeholder.txt -exec rm -rf {} +
	cp -r web/dist/* internal/server/dist/
	go build -o bin/danta ./cmd/danta
ifneq ($(UPX),)
	$(UPX) bin/danta
endif

# 安装前端依赖
web-install:
	cd web && pnpm install --frozen-lockfile

# 前端开发（vite dev 反代 /api → 127.0.0.1:8080）
web-dev:
	cd web && pnpm dev

# 仅构建前端并拷入 embed
web-build:
	cd web && pnpm build
	mkdir -p internal/server/dist
	find internal/server/dist -mindepth 1 ! -name placeholder.txt -exec rm -rf {} +
	cp -r web/dist/* internal/server/dist/

test:
	go test ./...

vet:
	go vet ./...

lint: vet

run:
	go run ./cmd/danta

clean:
	rm -rf bin web/dist internal/server/dist/assets
