BIN=build
SIPP=monsipp
PLOT=rtplot

all: bin sipp plot

bin:
	mkdir -p $(BIN)

sipp:
	go build -o $(BIN)/$(SIPP) $(SIPP).go

plot:
	go build -o $(BIN)/$(PLOT) $(PLOT).go

clean:
	rm -rf $(BIN)

install:
	/usr/bin/install $(BIN)/$(SIPP) /usr/local/bin/

uninstall:
	rm -f /usr/local/bin/$(SIPP)
