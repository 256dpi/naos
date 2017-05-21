cmake_minimum_required(VERSION 3.7)
project(nadk)

set(CMAKE_C_STANDARD 99)

include_directories(
        test/build/include
        test/xtensa-esp32-elf/xtensa-esp32-elf/include
        test/esp-idf/components/app_update/include
        test/esp-idf/components/bt/bluedroid/include
        test/esp-idf/components/bt/bluedroid/api/include
        test/esp-idf/components/bt/bluedroid/bta/include
        test/esp-idf/components/bt/bluedroid/btc/include
        test/esp-idf/components/bt/bluedroid/btcore/include
        test/esp-idf/components/bt/include
        test/esp-idf/components/esp32/include
        test/esp-idf/components/freertos/include
        test/esp-idf/components/log/include
        test/esp-idf/components/nvs_flash/include
        test/esp-idf/components/driver/include
        test/esp-idf/components/spi_flash/include
        test/esp-idf/components/tcpip_adapter/include
        test/esp-idf/components/lwip/include/lwip/port
        test/esp-idf/components/lwip/include/lwip
        test/components/esp-mqtt
        include)

set(SOURCE_FILES
        include/nadk.h
        src/ble.c
        src/ble.h
        src/device.c
        src/device.h
        src/general.c
        src/general.h
        src/led.c
        src/led.h
        src/manager.c
        src/manager.h
        src/mqtt.c
        src/mqtt.h
        src/nadk.c
        src/system.c
        src/system.h
        src/update.c
        src/update.h
        src/wifi.c
        src/wifi.h
        test/main/main.c)

add_library(nadk ${SOURCE_FILES})
