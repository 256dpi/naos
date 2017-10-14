#include <esp_partition.h>
#include <stdlib.h>

static const esp_partition_t* naos_coredump_partition() {
  // track the found partition
  static const esp_partition_t* p = NULL;

  // get partition if missing
  if (p == NULL) {
    p = esp_partition_find_first(ESP_PARTITION_TYPE_DATA, ESP_PARTITION_SUBTYPE_DATA_COREDUMP, NULL);
    if (p == NULL) {
      ESP_ERROR_CHECK(ESP_ERR_NOT_FOUND);
    }
  }

  return p;
}

uint32_t naos_coredump_size() {
  // read magic
  uint32_t magic;
  ESP_ERROR_CHECK(esp_partition_read(naos_coredump_partition(), 0, &magic, 4));

  // check magic
  if (magic != 0xE32C04ED) {
    return 0;
  }

  // read size
  uint32_t size = 0;
  ESP_ERROR_CHECK(esp_partition_read(naos_coredump_partition(), 4, &size, 4));

  return size;
}

void naos_coredump_read(uint32_t offset, uint32_t length, void* buf) {
  // read coredump chunk
  ESP_ERROR_CHECK(esp_partition_read(naos_coredump_partition(), 4 + offset, buf, length));
}

void naos_coredump_delete() {
  // overwrite the magic to "delete" the coredump
  uint32_t magic = 0;
  ESP_ERROR_CHECK(esp_partition_write(naos_coredump_partition(), 0, &magic, 4));
}
