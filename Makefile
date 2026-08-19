.PHONY: build web-dev web-build test lint vet run clean

# 完整构建：前端 build → 拷入 embed → go build
build:
	cd web && npm run build
	rm -rf internal/server/dist
	mkdir -p internal/server/dist
	cp -r web/dist/* internal/server/dist/
	go build -o bin/danta ./cmd/danta

# 前端开发（vite dev 反代 /api → 127.0.0.1:32682）
web-dev:
	cd web && npm run dev

# 仅构建前端并拷入 embed
web-build:
	cd web && npm run build
	rm -rf internal/server/dist
	mkdir -p internal/server/dist
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
