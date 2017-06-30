cmake_minimum_required(VERSION 3.7)
project(naos-esp)

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
        include/naos.h
        include/naos/utils.h
        src/ble.c
        src/ble.h
        src/manager.c
        src/manager.h
        src/mqtt.c
        src/mqtt.h
        src/naos.c
        src/naos.h
        src/system.c
        src/system.h
        src/task.c
        src/task.h
        src/update.c
        src/update.h
        src/utils.c
        src/utils.h
        src/wifi.c
        src/wifi.h
        test/main/main.c)

add_library(naos-esp ${SOURCE_FILES})
