GO ?= go
LINTER  ?= golangci-lint
ALIGNER ?= betteralign

.PHONY: test bench verify vet fmt fmt-check lint align align-fix check tidy download tools release-notes

check: fmt-check vet lint align test

fmt:
	gofmt -w .

fmt-check:
	@gofmt -l . | tee /dev/stderr | read; \
	if [ $$? -eq 0 ]; then \
		echo "gofmt: files need formatting"; \
		exit 1; \
	fi

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

bench:
	$(GO) test -run=^$$ -bench 'Benchmark' -benchmem

verify:
	$(GO) mod verify

tidy:
	$(GO) mod tidy

download:
	$(GO) mod download

lint:
	$(LINTER) run ./...

align:
	$(ALIGNER) ./...

align-fix:
	$(ALIGNER) -apply ./...

tools:
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	$(GO) install github.com/dkorunic/betteralign/cmd/betteralign@latest

release-notes:
	@awk '\
	/^<!--/,/^-->/ { next } \
	/^## \[[0-9]+\.[0-9]+\.[0-9]+\]/ { if (found) exit; found=1; next } \
	found { \
		if (/^## \[/) { exit } \
		if (/^$$/) { flush(); print; next } \
		if (/^\* / || /^- /) { flush(); buf=$$0; next } \
		if (/^###/ || /^\[/) { flush(); print; next } \
		sub(/^[ \t]+/, ""); sub(/[ \t]+$$/, ""); \
		if (buf != "") { buf = buf " " $$0 } else { buf = $$0 } \
		next \
	} \
	function flush() { if (buf != "") { print buf; buf = "" } } \
	END { flush() } \
	' CHANGELOG.md
