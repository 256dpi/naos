#include <naos/serial.h>
#include <naos/msg.h>
#include <naos/sys.h>
#include <string.h>

#include <esp_err.h>
#include <mbedtls/base64.h>

#define NAOS_SERIAL_BUFFER_SIZE 4096

typedef size_t (*naos_serial_read_t)(uint8_t* data, size_t len);

typedef struct {
  uint8_t* buffer;
  naos_serial_read_t read;
  uint8_t channel;
} naos_serial_decoder_t;

static bool naos_serial_encode(const uint8_t* data, size_t len, uint8_t* out_data, size_t* out_len) {
  // add magic
  memcpy(out_data, "\nNAOS!", 6);

  // encode message
  size_t n = 0;
  int ret = mbedtls_base64_encode(out_data + 6, NAOS_SERIAL_BUFFER_SIZE - 6, &n, data, len);
  if (ret != 0) {
    return false;
  }

  // add newline
  out_data[6 + n] = '\n';

  // set length
  *out_len = 7 + n;

  return true;
}

static bool naos_serial_decode(naos_serial_decoder_t decoder) {
  // prepare position
  size_t len = 0;
  size_t discard = 0;

  for (;;) {
    // discard buffer
    if (discard > 0) {
      memmove(decoder.buffer, decoder.buffer + discard, len - discard);
      len -= discard;
      decoder.buffer[len] = 0;
      discard = 0;
    }

    // fill buffer
    while (strchr((char*)decoder.buffer, '\n') == NULL) {
      size_t ret = decoder.read(decoder.buffer + len, NAOS_SERIAL_BUFFER_SIZE - len);
      len += ret;
      decoder.buffer[len] = 0;
      naos_delay(5);
    }

    /* got new line */

    // determine end
    uint8_t* end = (uint8_t*)strchr((char*)decoder.buffer, '\n');
    if (end == NULL) {
      ESP_ERROR_CHECK(ESP_FAIL);
      continue;
    }

    // check magic
    if (memcmp(decoder.buffer, "NAOS!", 5) != 0) {
      discard = end - decoder.buffer + 1;
      continue;
    }

    /* found magic, read message */

    // decode message
    size_t n = 0;
    int r = mbedtls_base64_decode(decoder.buffer, NAOS_SERIAL_BUFFER_SIZE, &n, decoder.buffer + 5,
                                  end - decoder.buffer - 5);
    if (r != 0) {
      discard = end - decoder.buffer + 1;
      continue;
    }

    // dispatch message
    naos_msg_dispatch(decoder.channel, decoder.buffer, n, NULL);

    // discard message
    discard = end - decoder.buffer + 1;
  }
}

/* STDIO Interface */

static uint8_t naos_serial_stdio_channel = 0;
static uint8_t naos_serial_stdio_input[NAOS_SERIAL_BUFFER_SIZE];
static uint8_t naos_serial_stdio_output[NAOS_SERIAL_BUFFER_SIZE];

static uint16_t naos_serial_mtu() { return 2560; }

static bool naos_serial_stdio_send(const uint8_t* data, size_t len, void* _) {
  // encode message
  size_t enc_len;
  if (!naos_serial_encode(data, len, naos_serial_stdio_output, &enc_len)) {
    return false;
  }

  // write message
  size_t ret = fwrite(naos_serial_stdio_output, 1, enc_len, stdout);
  if (ret != enc_len) {
    return false;
  }

  return true;
}

static size_t naos_serial_stdio_read_stdio(uint8_t* data, size_t len) {
  // read input
  size_t ret = fread(data, 1, len, stdin);

  return ret;
}

static void naos_serial_stdio_task() {
  // run decoder
  naos_serial_decode((naos_serial_decoder_t){
      .buffer = naos_serial_stdio_input,
      .read = naos_serial_stdio_read_stdio,
      .channel = naos_serial_stdio_channel,
  });
}

void naos_serial_init_stdio() {
  // register channel
  naos_serial_stdio_channel = naos_msg_register((naos_msg_channel_t){
      .name = "serial-stdio",
      .mtu = naos_serial_mtu,
      .send = naos_serial_stdio_send,
  });

  // start task
  naos_run("naos-serial-s", 4096, 1, naos_serial_stdio_task);
}

/* USB Interface */

#if CONFIG_SOC_USB_SERIAL_JTAG_SUPPORTED

#include <driver/usb_serial_jtag.h>

static uint8_t naos_serial_usb_channel = 0;
static uint8_t naos_serial_usb_input[NAOS_SERIAL_BUFFER_SIZE];
static uint8_t naos_serial_usb_output[NAOS_SERIAL_BUFFER_SIZE];

static bool naos_serial_usb_send(const uint8_t* data, size_t len, void* ctx) {
  // encode message
  size_t enc_len = 0;
  if (!naos_serial_encode(data, len, naos_serial_usb_output, &enc_len)) {
    return false;
  }

  // write message
  int ret = usb_serial_jtag_write_bytes(naos_serial_usb_output, enc_len, portMAX_DELAY);
  if (ret != enc_len) {
    return false;
  }

  return true;
}

static size_t naos_serial_usb_read(uint8_t* data, size_t len) {
  // read interface
  int ret = usb_serial_jtag_read_bytes(data, len, portMAX_DELAY);

  return (size_t)ret;
}

static void naos_serial_usb_task() {
  // run decoder
  naos_serial_decode((naos_serial_decoder_t){
      .buffer = naos_serial_usb_input,
      .read = naos_serial_usb_read,
      .channel = naos_serial_usb_channel,
  });
}

void naos_serial_init_usb() {
  // configure USB serial/JTAG driver
  usb_serial_jtag_driver_config_t config = USB_SERIAL_JTAG_DRIVER_CONFIG_DEFAULT();
  ESP_ERROR_CHECK(usb_serial_jtag_driver_install(&config));

  // register USB channel
  naos_serial_usb_channel = naos_msg_register((naos_msg_channel_t){
      .name = "serial-usb",
      .mtu = naos_serial_mtu,
      .send = naos_serial_usb_send,
  });

  // run task
  naos_run("naos-serial-u", 4096, 1, naos_serial_usb_task);
}

#endif  // CONFIG_SOC_USB_OTG_SUPPORTED
