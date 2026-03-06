BINARY := homedash
SRC := ./cmd/homedash
TOOLS_BIN := $(CURDIR)/.bin
GOLANGCI_LINT := $(TOOLS_BIN)/golangci-lint
GOLANGCI_LINT_VERSION ?= v2.11.1

.PHONY: build run clean lint lint-install

build:
	go build -o $(BINARY) $(SRC)

$(GOLANGCI_LINT):
	mkdir -p $(TOOLS_BIN)
	curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b $(TOOLS_BIN) $(GOLANGCI_LINT_VERSION)

lint-install: $(GOLANGCI_LINT)

lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)
