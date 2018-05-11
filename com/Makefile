UNAME := $(shell uname)

XTENSA_TOOLCHAIN := "xtensa-esp32-elf-linux64-1.22.0-80-g6c4433a-5.2.0.tar.gz"

ifeq ($(UNAME), Darwin)
XTENSA_TOOLCHAIN := "xtensa-esp32-elf-osx-1.22.0-80-g6c4433a-5.2.0.tar.gz"
endif

ESP_IDF_VERSION := "v3.0"
ESP_MQTT_VERSION := "v0.6.0"

test/xtensa-esp32-elf:
	wget https://dl.espressif.com/dl/$(XTENSA_TOOLCHAIN)
	cd test; tar -xzf ../$(XTENSA_TOOLCHAIN)
	rm *.tar.gz

test/esp-idf:
	git clone --recursive  https://github.com/espressif/esp-idf.git test/esp-idf
	cd test/esp-idf; git fetch; git checkout $(ESP_IDF_VERSION)
	cd test/esp-idf; git submodule update --recursive

test/components/esp-mqtt:
	git clone --recursive  https://github.com/256dpi/esp-mqtt.git test/components/esp-mqtt
	cd test/components/esp-mqtt; git fetch; git checkout $(ESP_MQTT_VERSION)
	cd test/components/esp-mqtt; git submodule update --recursive

update:
	cd test/esp-idf; git fetch; git checkout $(ESP_IDF_VERSION)
	cd test/esp-idf; git submodule update --recursive
	cd test/components/esp-mqtt; git fetch; git checkout $(ESP_MQTT_VERSION)
	cd test/components/esp-mqtt; git submodule update --recursive

clean: test/xtensa-esp32-elf test/esp-idf test/components/esp-mqtt
	export PATH=$(shell pwd)/test/xtensa-esp32-elf/bin:$$PATH; cd ./test; make clean

build: test/xtensa-esp32-elf test/esp-idf test/components/esp-mqtt
	export PATH=$(shell pwd)/test/xtensa-esp32-elf/bin:$$PATH; cd ./test; make

flash: test/xtensa-esp32-elf test/esp-idf test/components/esp-mqtt
	export PATH=$(shell pwd)/test/xtensa-esp32-elf/bin:$$PATH; cd ./test; make flash

erase: test/xtensa-esp32-elf test/esp-idf test/components/esp-mqtt
	export PATH=$(shell pwd)/test/xtensa-esp32-elf/bin:$$PATH; cd ./test; make erase_flash

monitor: test/xtensa-esp32-elf test/esp-idf test/components/esp-mqtt
	@clear
	miniterm.py /dev/cu.SLAB_USBtoUART 115200 --rts 0 --dtr 0 --raw --exit-char 99

idf-monitor: test/xtensa-esp32-elf test/esp-idf test/components/esp-mqtt
	export PATH=$(shell pwd)/test/xtensa-esp32-elf/bin:$$PATH; cd ./test; make monitor

config:
	export PATH=$(shell pwd)/test/xtensa-esp32-elf/bin:$$PATH; cd ./test; make menuconfig

info:
	export PATH=$(shell pwd)/test/xtensa-esp32-elf/bin:$$PATH; cd ./test; make size-components
	export PATH=$(shell pwd)/test/xtensa-esp32-elf/bin:$$PATH; cd ./test; make size

run: build flash monitor

fmt:
	clang-format -i ./src/*.c ./src/*.h -style="{BasedOnStyle: Google, ColumnLimit: 120}"
	clang-format -i ./include/*.h -style="{BasedOnStyle: Google, ColumnLimit: 120}"
	clang-format -i ./test/main/*.c -style="{BasedOnStyle: Google, ColumnLimit: 120}"