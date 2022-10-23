#ifndef NAOS_COREDUMP_H
#define NAOS_COREDUMP_H

uint32_t naos_coredump_size();
void naos_coredump_read(uint32_t offset, uint32_t length, void *buf);
void naos_coredump_delete();

#endif  // NAOS_COREDUMP_H
