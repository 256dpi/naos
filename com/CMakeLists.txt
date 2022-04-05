cmake_minimum_required(VERSION 3.7)
project(naos)

set(CMAKE_C_STANDARD 99)

include_directories(
        test/build/include
        test/esp-idf/components/app_update/include
        test/esp-idf/components/bt/include/esp32/include
        test/esp-idf/components/bt/host/bluedroid/api/include/api
        test/esp-idf/components/bt/host/bluedroid/bta/include/bta
        test/esp-idf/components/bt/host/bluedroid/btc/include/btc
        test/esp-idf/components/bt/host/bluedroid/common/include/common
        test/esp-idf/components/bt/host/bluedroid/hci/include/hci
        test/esp-idf/components/esp32/include
        test/esp-idf/components/esp_common/include
        test/esp-idf/components/esp_event/include
        test/esp-idf/components/esp_eth/include
        test/esp-idf/components/esp_hw_support/include
        test/esp-idf/components/esp_system/include
        test/esp-idf/components/esp_wifi/include
        test/esp-idf/components/freertos/include
        test/esp-idf/components/freertos/port/xtensa/include
        test/esp-idf/components/freertos/include/esp_additions/freertos
        test/esp-idf/components/log/include
        test/esp-idf/components/nvs_flash/include
        test/esp-idf/components/driver/include
        test/esp-idf/components/spi_flash/include
        test/esp-idf/components/tcpip_adapter/include
        test/esp-idf/components/lwip/lwip/src/include
        test/esp-idf/components/lwip/port/esp32/include
        test/esp-idf/components/soc/esp32/include
        test/esp-idf/components/soc/include
        test/components/esp-mqtt
        include)

set(SOURCE_FILES
        include/naos.h
        src/ble.c
        src/ble.h
        src/coredump.c
        src/coredump.h
        src/manager.c
        src/manager.h
        src/monitor.c
        src/monitor.h
        src/mqtt.c
        src/mqtt.h
        src/naos.c
        src/naos.h
        src/net.c
        src/net.h
        src/params.c
        src/params.h
        src/settings.c
        src/settings.h
        src/system.c
        src/system.h
        src/task.c
        src/task.h
        src/update.c
        src/update.h
        src/utils.c
        src/utils.h
        test/main/main.c
        test/main/eth.c)

add_library(naos ${SOURCE_FILES})
