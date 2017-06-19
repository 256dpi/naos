UNAME := $(shell uname)

XTENSA_TOOLCHAIN := "xtensa-esp32-elf-linux64-1.22.0-61-gab8375a-5.2.0.tar.gz"

ifeq ($(UNAME), Darwin)
XTENSA_TOOLCHAIN := "xtensa-esp32-elf-osx-1.22.0-61-gab8375a-5.2.0.tar.gz"
endif

fmt:
	clang-format -i ./src/*.c ./src/*.h -style="{BasedOnStyle: Google, ColumnLimit: 120}"
	clang-format -i ./include/*.h -style="{BasedOnStyle: Google, ColumnLimit: 120}"
	clang-format -i ./test/main/*.c -style="{BasedOnStyle: Google, ColumnLimit: 120}"

test/xtensa-esp32-elf:
	wget https://dl.espressif.com/dl/$(XTENSA_TOOLCHAIN)
	cd test; tar -xzf ../$(XTENSA_TOOLCHAIN)
	rm *.tar.gz

test/esp-idf:
	git clone --recursive --depth 1 https://github.com/espressif/esp-idf.git test/esp-idf

test/components/esp-mqtt:
	git clone --recursive --depth 1 https://github.com/256dpi/esp-mqtt.git test/components/esp-mqtt

build: test/xtensa-esp32-elf test/esp-idf test/components/esp-mqtt
	export PATH=$(shell pwd)/test/xtensa-esp32-elf/bin:$$PATH; cd ./test; make

flash: test/xtensa-esp32-elf test/esp-idf test/components/esp-mqtt
	export PATH=$(shell pwd)/test/xtensa-esp32-elf/bin:$$PATH; cd ./test; make flash

erase: test/xtensa-esp32-elf test/esp-idf test/components/esp-mqtt
	export PATH=$(shell pwd)/test/xtensa-esp32-elf/bin:$$PATH; cd ./test; make erase_flash

monitor: test/xtensa-esp32-elf test/esp-idf test/components/esp-mqtt
	@clear
	miniterm.py /dev/cu.SLAB_USBtoUART 115200 --rts 0 --dtr 0 --raw --exit-char 99

run: build flash monitor

update:
	cd test/esp-idf; git pull; git checkout master
	cd test/esp-idf/; git submodule update --recursive
	cd test/components/esp-mqtt/; git pull; git checkout master
	cd test/components/esp-mqtt/; git submodule update --recursive

version:
	@echo esp-idf:
	@cd test/esp-idf/; git rev-parse HEAD
	@echo esp-mqtt:
	@cd test/components/esp-mqtt/; git rev-parse HEAD
