BIN=build
NFS=nfs

all: bin nfs

bin:
	mkdir -p $(BIN)

nfs:
	go build -o $(BIN)/$(NFS) $(NFS).go

clean:
	rm -rf $(BIN)
