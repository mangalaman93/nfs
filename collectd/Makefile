BIN=build
PLUGINS_DIR=/usr/share/collectd/plugins
PLUGINS=cpu mem net

all:
	mkdir -p $(BIN)
	for plugin in $(PLUGINS); do go build -o $(BIN)/$$plugin $$plugin.go; done

install:
	mkdir -p $(PLUGINS_DIR)
	mv $(BIN)/* $(PLUGINS_DIR)/
