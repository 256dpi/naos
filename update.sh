#!/usr/bin/env bash

ESP_IDF_VERSION=$(cat ./tree/esp-idf.version)
ESP_MQTT_VERSION=$(cat ./tree/esp-mqtt.version)
ESP_OSC_VERSION=$(cat ./tree/esp-osc.version)

printf "\nCurrent Versions:\n"
printf "==> esp-idf: ${ESP_IDF_VERSION}\n"
printf "==> esp-mqtt: ${ESP_MQTT_VERSION}\n"
printf "==> esp-osc: ${ESP_OSC_VERSION}\n"

printf "\nUpdating naos tree...\n"
cd tree
cd components/esp-mqtt; git fetch; git checkout ${ESP_MQTT_VERSION}; git submodule update --recursive; cd ../..
cd components/esp-osc; git fetch; git checkout ${ESP_OSC_VERSION}; git submodule update --recursive; cd ../..
cd ..

printf "\nUpdating naos com...\n"
cd com
make update
