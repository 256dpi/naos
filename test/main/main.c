#include <stdio.h>

#include <nadk.h>
#include <nadk/mqtt.h>
#include <nadk/time.h>

static void setup() { nadk_subscribe("hello", 0); }

static void handle(const char *topic, const char *payload, unsigned int len) {
  printf("incoming: %s => %s\n", topic, payload);
}

static void loop() {
  nadk_publish_str("hello", "world", 0, false);
  nadk_sleep(1000);
}

static void terminate() {}

nadk_device_t device = {
    .name = "nadk-test", .setup_fn = setup, .handle_fn = handle, .loop_fn = loop, .terminate_fn = terminate,
};

void app_main() { nadk_init(&device); }
