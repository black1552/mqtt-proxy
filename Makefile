# Makefile — 构建内嵌控制台 + 单个 Go 二进制。
.PHONY: all frontend backend run migrate clean

all: frontend backend

frontend:
	cd admin && (test -d node_modules || npm install) && npm run build

backend:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o mqtt-proxy .

run: backend
	./mqtt-proxy

migrate:
	./mqtt-proxy migrate up

clean:
	rm -f mqtt-proxy mqtt-proxy.exe
	rm -rf admin/dist admin/node_modules
