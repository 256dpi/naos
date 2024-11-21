SHELL := /bin/bash

TARGET := "esp32" # run "reconfigure" when changed

ESP_IDF_VERSION := $(shell cat ../tree/esp-idf.version)
ESP_MQTT_VERSION := $(shell cat ../tree/esp-mqtt.version)
ESP_OSC_VERSION := $(shell cat ../tree/esp-osc.version)

prepare:
	rm -rf test/build test/esp-idf test/tools
	git clone --recursive  https://github.com/espressif/esp-idf.git test/esp-idf
	cd test/esp-idf; git fetch; git checkout $(ESP_IDF_VERSION)
	cd test/esp-idf; git submodule update --recursive --init
	git clone --recursive  https://github.com/256dpi/esp-mqtt.git test/components/esp-mqtt
	cd test/components/esp-mqtt; git fetch; git checkout $(ESP_MQTT_VERSION); git submodule update --recursive --init
	git clone --recursive  https://github.com/256dpi/esp-osc.git test/components/esp-osc
	cd test/components/esp-osc; git fetch; git checkout $(ESP_OSC_VERSION); git submodule update --recursive --init

update:
	cd test/esp-idf; git fetch; git checkout $(ESP_IDF_VERSION)
	cd test/esp-idf; git submodule update --recursive --init
	cd test/components/esp-mqtt; git fetch; git pull; git checkout $(ESP_MQTT_VERSION); git submodule update --recursive --init
	cd test/components/esp-osc; git fetch; git pull; git checkout $(ESP_OSC_VERSION); git submodule update --recursive --init

install:
	export IDF_TOOLS_PATH=$(shell pwd)/test/tools; cd test/esp-idf; ./install.sh all

clean:
	export IDF_TOOLS_PATH=$(shell pwd)/test/tools; . test/esp-idf/export.sh; cd ./test; idf.py clean

build:
	export IDF_TOOLS_PATH=$(shell pwd)/test/tools; . test/esp-idf/export.sh; cd ./test; idf.py build

flash:
	export IDF_TOOLS_PATH=$(shell pwd)/test/tools; . test/esp-idf/export.sh; cd ./test; idf.py flash

erase:
	export IDF_TOOLS_PATH=$(shell pwd)/test/tools; . test/esp-idf/export.sh; cd ./test; idf.py erase-flash

monitor:
	@echo "Exit monitor by pressing CTRL+T and CTRL+X."
	export IDF_TOOLS_PATH=$(shell pwd)/test/tools; . test/esp-idf/export.sh; cd ./test; idf.py monitor

simple-monitor:
	@clear
	miniterm.py /dev/cu.SLAB_USBtoUART 115200 --rts 0 --dtr 0 --raw --exit-char 99

config:
	export IDF_TOOLS_PATH=$(shell pwd)/test/tools; . test/esp-idf/export.sh; cd ./test; idf.py menuconfig

reconfigure:
	rm -f ./test/sdkconfig
	export IDF_TOOLS_PATH=$(shell pwd)/test/tools; . test/esp-idf/export.sh; cd ./test; idf.py fullclean
	export IDF_TOOLS_PATH=$(shell pwd)/test/tools; . test/esp-idf/export.sh; cd ./test; idf.py set-target $(TARGET)
	export IDF_TOOLS_PATH=$(shell pwd)/test/tools; . test/esp-idf/export.sh; cd ./test; idf.py reconfigure

info:
	export IDF_TOOLS_PATH=$(shell pwd)/test/tools; . test/esp-idf/export.sh; cd ./test; idf.py size-components
	export IDF_TOOLS_PATH=$(shell pwd)/test/tools; . test/esp-idf/export.sh; cd ./test; idf.py size

exec:
	export IDF_TOOLS_PATH=$(shell pwd)/test/tools; . test/esp-idf/export.sh; cd ./test; bash

debug:
	export IDF_TOOLS_PATH=$(shell pwd)/test/tools; . test/esp-idf/export.sh; cd ./test; ./esp-idf/components/espcoredump/espcoredump.py -p /dev/cu.SLAB_USBtoUART info_corefile ./build/naos.elf

run: build flash monitor

fmt:
	clang-format -i ./src/*.c ./src/*.h -style="{BasedOnStyle: Google, ColumnLimit: 120, SortIncludes: false}"
	clang-format -i ./include/*.h -style="{BasedOnStyle: Google, ColumnLimit: 120, SortIncludes: false}"
	clang-format -i ./test/main/*.c -style="{BasedOnStyle: Google, ColumnLimit: 120, SortIncludes: false}"
