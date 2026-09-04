.PHONY: check test vet fmt build clean web

check: fmt vet test build

fmt:
	@echo "[fmt]"
	@if [ -n "$$(gofmt -l . )" ]; then \
		echo "以下文件需要格式化:"; \
		gofmt -l .; \
		exit 1; \
	fi

vet:
	@echo "[vet]"
	go vet ./...

test:
	@echo "[test]"
	go test ./... -count=1 -timeout 60s

build:
	@echo "[build]"
	go build -o /dev/null .

web:
	@echo "[web]"
	cd web && pnpm build

clean:
	rm -f polarix
