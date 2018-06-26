#!/usr/bin/env bash

TOOLCHAIN_VERSION=$(cat ./tree/toolchain.version)
ESP_IDF_VERSION=$(cat ./tree/esp-idf.version)
ESP_MQTT_VERSION=$(cat ./tree/esp-mqtt.version)

printf "\nCurrent Versions:\n"
printf "==> toolchain: ${TOOLCHAIN_VERSION}\n"
printf "==> esp-idf: ${ESP_IDF_VERSION}\n"
printf "==> esp-mqtt: ${ESP_MQTT_VERSION}\n"

printf "\nUpdating naos tree...\n"
cd tree
cd esp-idf; git fetch; git checkout ${ESP_IDF_VERSION}; git submodule update --recursive; cd ..
cd components/esp-mqtt; git fetch; git checkout ${ESP_MQTT_VERSION}; git submodule update --recursive; cd ../..
cd ..

printf "\nCopying sdkconfig...\n"
cp com/test/sdkconfig tree/sdkconfig

printf "\nUpdating naos com...\n"
cd com
make update
