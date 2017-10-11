UNAME := $(shell uname)

XTENSA_TOOLCHAIN := "xtensa-esp32-elf-linux64-1.22.0-73-ge28a011-5.2.0.tar.gz"

ifeq ($(UNAME), Darwin)
XTENSA_TOOLCHAIN := "xtensa-esp32-elf-osx-1.22.0-73-ge28a011-5.2.0.tar.gz"
endif

ESP_IDF_VERSION := "e6afe28bafe5db5ab79fae213f2e8e1ccd9f937c"
ESP_MQTT_VERSION := "32d3ef69982638d945d8ce620bdad40a10c00d61"

test/xtensa-esp32-elf:
	wget https://dl.espressif.com/dl/$(XTENSA_TOOLCHAIN)
	cd test; tar -xzf ../$(XTENSA_TOOLCHAIN)
	rm *.tar.gz

test/esp-idf:
	git clone --recursive  https://github.com/espressif/esp-idf.git test/esp-idf
	cd test/esp-idf; git fetch; git checkout $(ESP_IDF_VERSION)
	cd test/esp-idf/; git submodule update --recursive

test/components/esp-mqtt:
	git clone --recursive  https://github.com/256dpi/esp-mqtt.git test/components/esp-mqtt
	cd test/components/esp-mqtt/; git fetch; git checkout $(ESP_MQTT_VERSION)
	cd test/components/esp-mqtt/; git submodule update --recursive

update:
	cd test/esp-idf; git fetch; git checkout $(ESP_IDF_VERSION)
	cd test/esp-idf/; git submodule update --recursive
	cd test/components/esp-mqtt/; git fetch; git checkout $(ESP_MQTT_VERSION)
	cd test/components/esp-mqtt/; git submodule update --recursive

build: test/xtensa-esp32-elf test/esp-idf test/components/esp-mqtt
	export PATH=$(shell pwd)/test/xtensa-esp32-elf/bin:$$PATH; cd ./test; make

flash: test/xtensa-esp32-elf test/esp-idf test/components/esp-mqtt
	export PATH=$(shell pwd)/test/xtensa-esp32-elf/bin:$$PATH; cd ./test; make flash

erase: test/xtensa-esp32-elf test/esp-idf test/components/esp-mqtt
	export PATH=$(shell pwd)/test/xtensa-esp32-elf/bin:$$PATH; cd ./test; make erase_flash

monitor: test/xtensa-esp32-elf test/esp-idf test/components/esp-mqtt
	@clear
	miniterm.py /dev/cu.SLAB_USBtoUART 115200 --rts 0 --dtr 0 --raw --exit-char 99

config:
	export PATH=$(shell pwd)/test/xtensa-esp32-elf/bin:$$PATH; cd ./test; make menuconfig

run: build flash monitor

fmt:
	clang-format -i ./src/*.c ./src/*.h -style="{BasedOnStyle: Google, ColumnLimit: 120}"
	clang-format -i ./include/*.h -style="{BasedOnStyle: Google, ColumnLimit: 120}"
	clang-format -i ./test/main/*.c -style="{BasedOnStyle: Google, ColumnLimit: 120}"
