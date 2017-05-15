#include <stdio.h>

#include <nadk.h>

static void setup() { nadk_subscribe("hello", 0, NADK_SCOPE_DEVICE); }

static void handle(const char *topic, const char *payload, unsigned int len, nadk_scope_t scope) {
  printf("incoming: %s => %s\n", topic, payload);
}

static void loop() {
  nadk_publish_str("hello", "world", 0, false, NADK_SCOPE_DEVICE);
  nadk_sleep(1000);
}

static void terminate() {}

nadk_device_t device = {
    .type = "nadk-test", .setup_fn = setup, .handle_fn = handle, .loop_fn = loop, .terminate_fn = terminate,
};

void app_main() { nadk_init(&device); }
