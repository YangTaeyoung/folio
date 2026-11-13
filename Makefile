VERSION ?= dev
LDFLAGS = -s -w -X main.version=$(VERSION)
.PHONY: build web test release docker clean
web:
	cd web && npm ci && npm run build
	mkdir -p internal/webassets/dist
	find internal/webassets/dist -depth -mindepth 1 ! -name .gitkeep -delete
	cp -R web/dist/. internal/webassets/dist/
build: web
	mkdir -p dist
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o dist/folio ./cmd/folio
test:
	go test ./...
	cd web && npm run typecheck
release: web
	./scripts/release.sh $(VERSION)
docker:
	docker build --build-arg VERSION=$(VERSION) -t folio:$(VERSION) .
clean:
	rm -rf dist web/dist internal/webassets/dist/assets
	rm -f internal/webassets/dist/index.html internal/webassets/dist/favicon.svg
