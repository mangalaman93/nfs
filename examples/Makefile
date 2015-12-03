BIN=build
SRC=$(shell ls *.go)
OBJ=$(patsubst %.go,%,$(SRC))
GO=$(shell which go)

all: dir compile

dir:
	mkdir -p $(BIN)

compile: $(OBJ)

%: %.go
	$(GO) build -o $(BIN)/$@ $<

clean:
	rm -rf $(BIN)

.PHONY: all dir clean compile
