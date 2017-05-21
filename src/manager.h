#ifndef _NADK_MANAGER_H
#define _NADK_MANAGER_H

#include <stdbool.h>

#include <nadk.h>

void nadk_manager_setup();

bool nadk_manager_handle(const char* topic, const char* payload, unsigned int len, nadk_scope_t scope);

void nadk_manager_terminate();

#endif  // _NADK_MANAGER_H
