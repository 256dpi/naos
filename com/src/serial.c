#include <naos/msg.h>
#include <naos/sys.h>
#include <string.h>

#include <stdio.h>
#include <mbedtls/base64.h>

#define NAOS_SERIAL_BUFFER_SIZE 4096

uint8_t naos_serial_channel = 0;
uint8_t naos_serial_output[NAOS_SERIAL_BUFFER_SIZE + 8];
uint8_t naos_serial_input[NAOS_SERIAL_BUFFER_SIZE];

static bool naos_serial_send(const uint8_t* data, size_t len, void* _) {
  // add magic
  memcpy(naos_serial_output, "\nNAOS!", 6);

  // encode message
  size_t n = 0;
  int ret = mbedtls_base64_encode(naos_serial_output + 6, NAOS_SERIAL_BUFFER_SIZE - 6, &n, data, len);
  if (ret != 0) {
    return false;
  }

  // add newline
  naos_serial_output[6 + n] = '\n';

  // write message
  ret = (int)fwrite(naos_serial_output, 1, 7 + n, stdout);
  if (ret != 7 + n) {
    return false;
  }

  return true;
}

static void naos_serial_task() {
  // prepare position
  size_t len = 0;
  size_t discard = 0;

  for (;;) {
    // discard buffer
    if (discard > 0) {
      memmove(naos_serial_input, naos_serial_input + discard, len - discard);
      len -= discard;
      naos_serial_input[len] = 0;
      discard = 0;
    }

    // fill buffer
    while (strchr((char*)naos_serial_input, '\n') == NULL) {
      size_t ret = fread(naos_serial_input + len, 1, NAOS_SERIAL_BUFFER_SIZE - len, stdin);
      len += ret;
      naos_serial_input[len] = 0;
      naos_delay(5);
    }

    /* got new line */

    // determine end
    uint8_t* end = (uint8_t*)strchr((char*)naos_serial_input, '\n');
    if (end == NULL) {
      ESP_ERROR_CHECK(ESP_FAIL);
      continue;
    }

    // check magic
    if (memcmp(naos_serial_input, "NAOS!", 5) != 0) {
      discard = end - naos_serial_input + 1;
      continue;
    }

    /* found magic, read message */

    // decode message
    size_t n = 0;
    int r = mbedtls_base64_decode(naos_serial_input, NAOS_SERIAL_BUFFER_SIZE, &n, naos_serial_input + 5,
                                  end - naos_serial_input - 5);
    if (r != 0) {
      discard = end - naos_serial_input + 1;
      continue;
    }

    // dispatch message
    naos_msg_dispatch(naos_serial_channel, naos_serial_input, n, NULL);

    // discard message
    discard = end - naos_serial_input + 1;
  }
}

void naos_serial_init() {
  // register channel
  naos_serial_channel = naos_msg_register((naos_msg_channel_t){
      .name = "serial",
      .mtu = NAOS_SERIAL_BUFFER_SIZE,
      .send = naos_serial_send,
  });

  // start task
  naos_run("serial", 4096, 1, naos_serial_task);
}
