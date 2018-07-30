GOCMD = go
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean
GOTEST = $(GOCMD) test
GOVET = $(GOCMD) vet
GOFMT = $(GOCMD) fmt
GOGET = $(GOCMD) get
BINDIR = bin
MACHINE = $(shell uname -s | tr '[A-Z]' '[a-z]')

# project specific definitions
PROJECT=pdf-ws
SRCDIR=cmd/$(PROJECT)

build: build-$(MACHINE)
	ln -s $(PROJECT).$(MACHINE) $(BINDIR)/$(PROJECT)

all: clean dep build

build-darwin:
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -a -o $(BINDIR)/$(PROJECT).$(MACHINE) $(SRCDIR)/*.go

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -a -installsuffix cgo -o $(BINDIR)/$(PROJECT).$(MACHINE) $(SRCDIR)/*.go

fmt:
	(cd $(SRCDIR) && $(GOFMT))

vet:
	(cd $(SRCDIR) && $(GOVET))

clean:
	$(GOCLEAN)
	rm -rf $(BINDIR)

dep:
	dep ensure
	dep status
