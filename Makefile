BIN := bin
WORKER := $(BIN)/needle-worker
NB := $(BIN)/nb
DAEMON := $(BIN)/notebotd

.PHONY: all worker nb daemon web install test tidy clean
all: worker nb daemon

web:
	cd web && npm install --no-audit --no-fund && npm run build

worker:
	mkdir -p $(BIN)
	go build -o $(WORKER) ./needle/cmd/needle-worker

nb: worker
	mkdir -p $(BIN)
	go build -o $(NB) ./cmd/nb

daemon: worker
	mkdir -p $(BIN)
	go build -o $(DAEMON) ./cmd/notebotd

PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin

install: nb daemon
	mkdir -p $(BINDIR)
	cp $(NB) $(DAEMON) $(WORKER) $(BINDIR)/
	@echo "installed nb, notebotd, needle-worker to $(BINDIR) (ensure it is on PATH)"

test:
	go test ./... -short

tidy:
	go mod tidy

clean:
	rm -rf $(BIN)
