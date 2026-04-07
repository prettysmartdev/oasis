BINARY_CONTROLLER := ./bin/controller
BINARY_CLI        := ./bin/oasis
IMAGE_TAG         := oasis:latest
VERSION           := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS           := -ldflags "-X main.version=$(VERSION)"

GOLANGCI_LINT_VERSION := v1.57.2

.PHONY: install-tools build build-webapp build-cli build-docker generate-icons run test lint \
        test-integration _build-controller _build-cli

## install-tools: Install golangci-lint into the local Go bin path.
install-tools:
	@echo "==> Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
	    | sh -s -- -b "$$(go env GOPATH)/bin" $(GOLANGCI_LINT_VERSION)
	@echo "==> Done. Ensure $$GOPATH/bin is on your PATH."

## build: Build all components (webapp + Go binaries).
build: build-webapp _build-controller _build-cli

## build-webapp: Build the Next.js static export.
build-webapp:
	npm --prefix webapp ci
	npm --prefix webapp run build

## generate-icons: Generate PWA icon PNGs from webapp/public/icons/icon.svg using sharp.
generate-icons:
	node -e "\
	const sharp = require('./webapp/node_modules/sharp');\
	const fs = require('fs');\
	const svg = fs.readFileSync('./webapp/public/icons/icon.svg');\
	const outDir = './webapp/public/icons';\
	(async () => {\
	  await sharp(svg).resize(192).png().toFile(outDir+'/icon-192.png');\
	  await sharp(svg).resize(512).png().toFile(outDir+'/icon-512.png');\
	  const bg192 = await sharp(Buffer.from('<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"192\" height=\"192\"><circle cx=\"96\" cy=\"96\" r=\"96\" fill=\"#0f172a\"/></svg>')).png().toBuffer();\
	  const fg192 = await sharp(svg).resize(154).png().toBuffer();\
	  await sharp(bg192).composite([{input:fg192,gravity:'center'}]).resize(192).png().toFile(outDir+'/icon-maskable-192.png');\
	  const bg512 = await sharp(Buffer.from('<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"512\" height=\"512\"><circle cx=\"256\" cy=\"256\" r=\"256\" fill=\"#0f172a\"/></svg>')).png().toBuffer();\
	  const fg512 = await sharp(svg).resize(410).png().toBuffer();\
	  await sharp(bg512).composite([{input:fg512,gravity:'center'}]).resize(512).png().toFile(outDir+'/icon-maskable-512.png');\
	  await sharp(svg).resize(180).png().toFile('./webapp/public/apple-touch-icon.png');\
	  console.log('Icons generated.');\
	})().catch(e=>{console.error(e);process.exit(1);});\
	"

## build-docker: Build the Docker image.
build-docker: generate-icons
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE_TAG) .

## run: Run the latest built image.
run:
	docker run --rm \
	    -p 127.0.0.1:04515:04515 \
	    -v oasis-db:/data/db \
	    -v oasis-ts-state:/data/ts-state \
	    $(IMAGE_TAG)

## build-cli: Build the CLI binary only → ./bin/oasis.
build-cli: _build-cli

_build-controller: $(shell find cmd/controller internal -name '*.go')
	@mkdir -p bin
	CGO_ENABLED=0 go build -mod=mod $(LDFLAGS) -o $(BINARY_CONTROLLER) ./cmd/controller

_build-cli: $(shell find cmd/oasis internal -name '*.go')
	@mkdir -p bin
	CGO_ENABLED=0 go build -mod=mod $(LDFLAGS) -o $(BINARY_CLI) ./cmd/oasis

## test: Run Go unit tests (with race detector) and web unit tests.
test:
	go test -race ./...
	npm --prefix webapp test -- --ci

## lint: Run golangci-lint on Go code and tsc + next lint on the webapp.
lint:
	golangci-lint run ./...
	npm --prefix webapp run lint

## test-integration: Run integration tests via Docker Compose.
test-integration:
	docker compose -f docker-compose.dev.yml up --abort-on-container-exit
