#include <stdio.h>

#include <nadk.h>

static void setup() { nadk_subscribe("hello", 0, NADK_LOCAL); }

static void handle(const char *topic, const char *payload, unsigned int len, nadk_scope_t scope) {
  printf("incoming: %s => %s\n", topic, payload);
}

static void loop() {
  nadk_publish_str("hello", "world", 0, false, NADK_LOCAL);
  nadk_sleep(1000);
}

static void terminate() {}

static nadk_config_t config = {
    .device_type = "nadk-test",
    .firmware_version = "0.0.1",
    .setup = setup,
    .handle = handle,
    .loop = loop,
    .terminate = terminate,
};

void app_main() { nadk_init(&config); }
