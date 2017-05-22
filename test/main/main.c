#include <stdio.h>

#include <nadk.h>

static void setup() { nadk_subscribe("hello", 0, NADK_LOCAL); }

static void handle(const char *topic, const char *payload, unsigned int len, nadk_scope_t scope) {
  printf("incoming: %s => %s\n", topic, payload);
}

static void loop() { nadk_publish_str("hello", "world", 0, false, NADK_LOCAL); }

static void terminate() {}

static nadk_config_t config = {
    .device_type = "nadk-test",
    .firmware_version = "0.0.1",
    .online_callback = setup,
    .message_callback = handle,
    .loop_callback = loop,
    .loop_interval = 1000,
    .offline_callback = terminate,
};

void app_main() { nadk_init(&config); }
