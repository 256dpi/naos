#include <naos/udp.h>
#include <naos/sys.h>
#include <naos/msg.h>

#include <sys/socket.h>
#include <mdns.h>

#define NAOS_UDP_MTU 512
#define NAOS_UDP_CONTEXTS 16

typedef struct {
  struct sockaddr addr;
} naos_udp_context_t;

static int naos_udp_port = 0;
static uint8_t naos_udp_channel = 0;
static int naos_udp_socket = 0;
static naos_udp_context_t naos_udp_contexts[NAOS_UDP_CONTEXTS] = {0};
static uint16_t naos_udp_context_num = 0;

static void naos_udp_server() {
  // create UDP socket
  naos_udp_socket = socket(AF_INET, SOCK_DGRAM, IPPROTO_IP);
  if (naos_udp_socket < 0) {
    ESP_LOGE("UDP", "naos_udp_server: failed to create socket");
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // prepare local address
  struct sockaddr_in local;
  local.sin_addr.s_addr = htonl(INADDR_ANY);
  local.sin_family = AF_INET;
  local.sin_port = htons(naos_udp_port);

  // bind socket
  int ret = bind(naos_udp_socket, (struct sockaddr *)&local, sizeof(local));
  if (ret < 0) {
    ESP_LOGE("UDP", "naos_udp_server: failed to bind socket");
    ESP_ERROR_CHECK(ESP_FAIL);
  }

  // prepare buffer
  uint8_t buffer[NAOS_UDP_MTU];

  for (;;) {
    // prepare remote address
    struct sockaddr remote;
    socklen_t socklen = sizeof(remote);

    // receive next message
    ssize_t len = recvfrom(naos_udp_socket, buffer, sizeof(buffer), 0, &remote, &socklen);
    if (len < 0) {
      ESP_LOGE("UDP", "naos_udp_server: failed to receive message");
      ESP_ERROR_CHECK(ESP_FAIL);
    }

    // get context
    naos_udp_context_t *context = &naos_udp_contexts[naos_udp_context_num++];
    if (naos_udp_context_num >= NAOS_UDP_CONTEXTS) {
      naos_udp_context_num = 0;
    }

    // set address
    context->addr = remote;

    // dispatch message
    naos_msg_dispatch(naos_udp_channel, buffer, len, context);
  }
}

static bool naos_udp_send(const uint8_t *data, size_t len, void *ctx) {
  // get context
  naos_udp_context_t *context = ctx;

  // send message
  ssize_t err = sendto(naos_udp_socket, data, len, 0, &context->addr, sizeof(context->addr));
  if (err < 0) {
    return false;
  }

  return true;
}

void naos_udp_init(int port) {
  // store port
  naos_udp_port = port;

  // create channel
  naos_udp_channel = naos_msg_register((naos_msg_channel_t){
      .name = "udp",
      .mtu = NAOS_UDP_MTU,
      .send = naos_udp_send,
  });

  // run server
  naos_run("udp", 4096, 1, naos_udp_server);

  /* run mDNS server */

  // get mac address
  uint64_t mac = 0;
  ESP_ERROR_CHECK(esp_base_mac_addr_get((uint8_t*)&mac));

  // initialize mDNS service
  ESP_ERROR_CHECK(mdns_init());

  // generate hostname
  char hostname[32];
  snprintf(hostname, sizeof(hostname), "naos-%llu", mac);

  // set hostname
  ESP_ERROR_CHECK(mdns_hostname_set(hostname));

  // add dummy service
  mdns_service_add(NULL, "_naos", "_udp", port, NULL, 0);
}
