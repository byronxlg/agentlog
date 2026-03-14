.PHONY: build test lint clean

BINDIR := bin

build: $(BINDIR)/agentlog $(BINDIR)/agentlogd

$(BINDIR)/agentlog:
	go build -o $(BINDIR)/agentlog ./cmd/agentlog

$(BINDIR)/agentlogd:
	go build -o $(BINDIR)/agentlogd ./cmd/agentlogd

test:
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BINDIR)
