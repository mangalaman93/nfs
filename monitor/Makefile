BIN=build
MON=moncont
PLOT=rtplot

all: bin sipp plot

bin:
	mkdir -p $(BIN)

sipp:
	go build -o $(BIN)/$(MON) $(MON).go

plot:
	go build -o $(BIN)/$(PLOT) $(PLOT).go

clean:
	rm -rf $(BIN)

install:
	/usr/bin/install $(BIN)/$(MON) /usr/local/bin/

uninstall:
	rm -f /usr/local/bin/$(MON)
