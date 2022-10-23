#ifndef _NAOS_COREDUMP_H
#define _NAOS_COREDUMP_H

uint32_t naos_coredump_size();
void naos_coredump_read(uint32_t offset, uint32_t length, void *buf);
void naos_coredump_delete();

#endif  // _NAOS_COREDUMP_H
