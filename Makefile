BIN=$(GOPATH)/bin
COMMON=common
NFH=nfh
NFS=nfs

all: common nfh nfs

common:
	cd $(COMMON) && go install

nfh:
	cd $(NFH) && go get && go install

nfs:
	cd $(NFS) && go get && go install

clean:
	rm -rf $(BIN)/nfh $(BIN)/nfs

.PHONY: all clean common nfh nfs
