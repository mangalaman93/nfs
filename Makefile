BIN=$(GOPATH)/bin
COMMON=common
LINUX=linux
DOCKER=docker
NFH=nfh
NFS=nfs
NF=nf

all: common linux docker nfh nfs

common:
	cd $(COMMON) && go install

linux:
	cd $(LINUX) && go install

docker:
	cd $(DOCKER) && go install

nfh:
	cd $(NFH) && go get && go install

nfs:
	cd $(NFS) && go get && go install

nf:
	cd $(NF) && go get && go install
	mv $(BIN)/nf $(BIN)/rtplot

clean:
	rm -rf $(BIN)/nfh $(BIN)/nfs

.PHONY: all clean common linux docker nfh nfs nf
