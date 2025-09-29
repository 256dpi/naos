#include <naos/serial.h>
#include <naos/msg.h>
#include <naos/sys.h>
#include <string.h>

#include <esp_err.h>
#include <esp_heap_caps.h>
#include <mbedtls/base64.h>
#include <driver/uart.h>
#include <driver/uart_vfs.h>
#include <driver/usb_serial_jtag.h>
#include <driver/usb_serial_jtag_vfs.h>

#define NAOS_SERIAL_BS CONFIG_NAOS_SERIAL_BUFFER_SIZE

/* Encoder */

static bool naos_serial_encode(const uint8_t* data, size_t len, uint8_t* out_data, size_t* out_len) {
  // add magic
  memcpy(out_data, "\nNAOS!", 6);

  // encode message
  size_t n = 0;
  int ret = mbedtls_base64_encode(out_data + 6, NAOS_SERIAL_BS - 7, &n, data, len);
  if (ret != 0) {
    return false;
  }

  // add newline
  out_data[6 + n] = '\n';

  // set length
  *out_len = 7 + n;

  return true;
}

/* Decoder */

typedef size_t (*naos_serial_read_t)(uint8_t* data, size_t len, void* ctx);

typedef struct {
  void* ctx;
  uint8_t* buffer;
  naos_serial_read_t read;
  uint8_t channel;
} naos_serial_decoder_t;

static void naos_serial_decode(naos_serial_decoder_t decoder) {
  // prepare state
  size_t len = 0;
  size_t discard = 0;

  // clear buffer
  decoder.buffer[0] = 0;

  for (;;) {
    // discard trailing newlines
    if (discard < len && decoder.buffer[discard] == '\n') {
      discard++;
    }

    // discard buffer
    if (discard > 0) {
      memmove(decoder.buffer, decoder.buffer + discard, len - discard);
      len -= discard;
      decoder.buffer[len] = 0;
      discard = 0;
    }

    // prepare end
    uint8_t* end = NULL;

    for (;;) {
      // stop if we have a newline in buffer (exclude first byte)
      if (len > 0) {
        end = memchr(decoder.buffer + 1, '\n', len - 1);
        if (end != NULL) {
          break;
        }
      }

      // get remaining buffer space
      size_t space = NAOS_SERIAL_BS - 1 - len;

      // clear buffer on overflow
      if (space == 0) {
        len = 0;
        decoder.buffer[0] = 0;
        continue;
      }

      // read into buffer
      size_t ret = decoder.read(decoder.buffer + len, space, decoder.ctx);
      if (ret == 0) {
        naos_delay(5);
        continue;
      }

      // increment
      len += ret;
      decoder.buffer[len] = 0;
    }

    /* got new line */

    // get line length
    size_t line_len = (size_t)(end - decoder.buffer);

    // discard line in any case
    discard = line_len + 1;

    // determine offset
    size_t offset = 0;
    if (decoder.buffer[0] == '\n') {
      offset = 1;
    }

    // check magic
    if (len < offset + 5 || memcmp(decoder.buffer + offset, "NAOS!", 5) != 0) {
      continue;
    }

    /* found magic, read message */

    // decode message
    size_t n = 0;
    int r =
        mbedtls_base64_decode(decoder.buffer, NAOS_SERIAL_BS, &n, decoder.buffer + offset + 5, line_len - offset - 5);
    if (r != 0) {
      continue;
    }

    // dispatch message
    naos_msg_dispatch(decoder.channel, decoder.buffer, n, decoder.ctx);
  }
}

/* Common */

static naos_mutex_t naos_serial_mutex = NULL;
static void* naos_serial_output = NULL;

static void* naos_serial_alloc() {
#ifdef CONFIG_SPIRAM
  void* buf = heap_caps_malloc_prefer(NAOS_SERIAL_BUFFER_SIZE, 2, MALLOC_CAP_SPIRAM, MALLOC_CAP_DEFAULT);
#else
  void* buf = malloc(NAOS_SERIAL_BS);
#endif
  if (buf == NULL) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  return buf;
}

static void naos_serial_init() {
  // ensure mutex
  if (naos_serial_mutex == NULL) {
    naos_serial_mutex = naos_mutex();
  }

  // ensure shared output buffer
  if (naos_serial_output == NULL) {
    naos_serial_output = naos_serial_alloc();
  }
}

static uint16_t naos_serial_mtu() {
  // max base64 encoding size
  return NAOS_SERIAL_BS / 5 * 3;
}

typedef struct {
  void* input;
  void* output;
} naos_serial_vfs_t;

static bool naos_serial_vfs_send(const uint8_t* data, size_t len, void* ctx) {
  // get context
  naos_serial_vfs_t* vfs = ctx;

  // acquire mutex
  naos_lock(naos_serial_mutex);

  // encode message
  size_t enc_len;
  if (!naos_serial_encode(data, len, naos_serial_output, &enc_len)) {
    naos_unlock(naos_serial_mutex);
    return false;
  }

  // write message
  size_t ret = fwrite(naos_serial_output, 1, enc_len, vfs->output);
  if (ret != enc_len) {
    naos_unlock(naos_serial_mutex);
    return false;
  }

  // release mutex
  naos_unlock(naos_serial_mutex);

  return true;
}

static size_t naos_serial_vfs_read(uint8_t* data, size_t len, void* ctx) {
  // get context
  naos_serial_vfs_t* vfs = ctx;

  // read input
  size_t ret = fread(data, 1, len, vfs->input);

  return ret;
}

/* STDIO Interface */

static naos_serial_vfs_t naos_serial_stdio_vfs;
static uint8_t naos_serial_stdio_channel = 0;
static void* naos_serial_stdio_input = NULL;

static void naos_serial_stdio_task() {
  // run decoder
  naos_serial_decode((naos_serial_decoder_t){
      .ctx = &naos_serial_stdio_vfs,
      .buffer = naos_serial_stdio_input,
      .read = naos_serial_vfs_read,
      .channel = naos_serial_stdio_channel,
  });
}

void naos_serial_init_stdio() {
  // init serial
  naos_serial_init();

  // prepare VFS context
  naos_serial_stdio_vfs.input = stdin;
  naos_serial_stdio_vfs.output = stdout;

  // allocate input buffer
  naos_serial_stdio_input = naos_serial_alloc();

  // register channel
  naos_serial_stdio_channel = naos_msg_register((naos_msg_channel_t){
      .name = "serial-stdio",
      .mtu = naos_serial_mtu,
      .send = naos_serial_vfs_send,
  });

  // start task
  naos_run("naos-srl-stdio", 4096, 1, naos_serial_stdio_task);
}

void naos_serial_init_stdio_uart() {
  // drain outputs
  fflush(stdout);
  fflush(stderr);

  // disable buffering
  setvbuf(stdout, NULL, _IONBF, 0);
  setvbuf(stderr, NULL, _IONBF, 0);
  setvbuf(stdin, NULL, _IONBF, 0);

  // configure UART parameters
  ESP_ERROR_CHECK(uart_vfs_dev_port_set_rx_line_endings(CONFIG_ESP_CONSOLE_UART_NUM, ESP_LINE_ENDINGS_CRLF));
  ESP_ERROR_CHECK(uart_vfs_dev_port_set_tx_line_endings(CONFIG_ESP_CONSOLE_UART_NUM, ESP_LINE_ENDINGS_LF));

  // prepare UART config
  const uart_config_t uart_config = {
      .baud_rate = CONFIG_ESP_CONSOLE_UART_BAUDRATE,
      .data_bits = UART_DATA_8_BITS,
      .parity = UART_PARITY_DISABLE,
      .stop_bits = UART_STOP_BITS_1,
      .flow_ctrl = UART_HW_FLOWCTRL_DISABLE,
      .source_clk = UART_SCLK_DEFAULT,
  };

  // install UART driver
  ESP_ERROR_CHECK(uart_driver_install(CONFIG_ESP_CONSOLE_UART_NUM, 256, 256, 0, NULL, 0));
  ESP_ERROR_CHECK(uart_param_config(CONFIG_ESP_CONSOLE_UART_NUM, &uart_config));

  // enable VFS driver
  uart_vfs_dev_use_driver(CONFIG_ESP_CONSOLE_UART_NUM);

  // initialize stdio IO
  naos_serial_init_stdio();
}

/* Secondary Interface */

static naos_serial_vfs_t naos_serial_secio_vfs;
static uint8_t naos_serial_secio_channel = 0;
static void* naos_serial_secio_input = NULL;

static void naos_serial_secio_task() {
  // run decoder
  naos_serial_decode((naos_serial_decoder_t){
      .ctx = &naos_serial_secio_vfs,
      .buffer = naos_serial_secio_input,
      .read = naos_serial_vfs_read,
      .channel = naos_serial_secio_channel,
  });
}

void naos_serial_init_secio() {
  // init serial
  naos_serial_init();

  // open secondary stream
  FILE* stream = fopen("/dev/secondary", "r+");
  if (stream == NULL) {
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // prepare VFS context
  naos_serial_secio_vfs.input = stream;
  naos_serial_secio_vfs.output = stream;

  // allocate input buffer
  naos_serial_secio_input = naos_serial_alloc();

  // register channel
  naos_serial_secio_channel = naos_msg_register((naos_msg_channel_t){
      .name = "serial-secio",
      .mtu = naos_serial_mtu,
      .send = naos_serial_vfs_send,
  });

  // start task
  naos_run("naos-srl-secio", 4096, 1, naos_serial_secio_task);
}

void naos_serial_init_secio_usj() {
  // configure parameters
  usb_serial_jtag_vfs_set_rx_line_endings(ESP_LINE_ENDINGS_CRLF);
  usb_serial_jtag_vfs_set_tx_line_endings(ESP_LINE_ENDINGS_LF);

  // configure driver
  usb_serial_jtag_driver_config_t config = USB_SERIAL_JTAG_DRIVER_CONFIG_DEFAULT();
  ESP_ERROR_CHECK(usb_serial_jtag_driver_install(&config));

  // upgrade VFS driver
  ESP_ERROR_CHECK(usb_serial_jtag_vfs_register());
  usb_serial_jtag_vfs_use_driver();

  // initialize secondary IO
  naos_serial_init_secio();
}

/* USB Interface */

#if CONFIG_SOC_USB_SERIAL_JTAG_SUPPORTED

static uint8_t naos_serial_usj_channel = 0;
static void* naos_serial_usj_input = NULL;

static bool naos_serial_usj_send(const uint8_t* data, size_t len, void* _) {
  // acquire mutex
  naos_lock(naos_serial_mutex);

  // encode message
  size_t enc_len = 0;
  if (!naos_serial_encode(data, len, naos_serial_output, &enc_len)) {
    naos_unlock(naos_serial_mutex);
    return false;
  }

  // write message
  int ret = usb_serial_jtag_write_bytes(naos_serial_output, enc_len, portMAX_DELAY);
  if (ret != enc_len) {
    naos_unlock(naos_serial_mutex);
    return false;
  }

  // release mutex
  naos_unlock(naos_serial_mutex);

  return true;
}

static size_t naos_serial_usj_read(uint8_t* data, size_t len, void* _) {
  // read interface
  int ret = usb_serial_jtag_read_bytes(data, len, portMAX_DELAY);

  return (size_t)ret;
}

static void naos_serial_usj_task() {
  // run decoder
  naos_serial_decode((naos_serial_decoder_t){
      .buffer = naos_serial_usj_input,
      .read = naos_serial_usj_read,
      .channel = naos_serial_usj_channel,
  });
}

void naos_serial_init_usj() {
  // init serial
  naos_serial_init();

  // allocate input buffer
  naos_serial_usj_input = naos_serial_alloc();

  // configure USB serial/JTAG driver
  usb_serial_jtag_driver_config_t config = USB_SERIAL_JTAG_DRIVER_CONFIG_DEFAULT();
  ESP_ERROR_CHECK(usb_serial_jtag_driver_install(&config));

  // register USB channel
  naos_serial_usj_channel = naos_msg_register((naos_msg_channel_t){
      .name = "serial-usb",
      .mtu = naos_serial_mtu,
      .send = naos_serial_usj_send,
  });

  // run task
  naos_run("naos-srl-usb", 4096, 1, naos_serial_usj_task);
}

#endif  // CONFIG_SOC_USB_SERIAL_JTAG_SUPPORTED
