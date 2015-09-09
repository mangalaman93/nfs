BIN=build
SIPP=monsipp

all:
	mkdir -p $(BIN)
	go build -o $(BIN)/$(SIPP) $(SIPP).go

install:
	/usr/bin/install $(BIN)/$(SIPP) /usr/local/bin/

uninstall:
	rm -f /usr/local/bin/$(SIPP)
