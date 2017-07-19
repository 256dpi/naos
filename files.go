package naos

// TODO: Read files from 'naos-tree'.

const mainSourceFile = `#include <naos.h>

static naos_config_t config = {.device_type = "my-device",
                               .firmware_version = "0.0.1"};

void app_main() {
  // initialize naos
  naos_init(&config);
}
`

const projectCMakeListsFile = `# minimal required cmake version
cmake_minimum_required(VERSION 3.7)

# you can set your own project name
project(naos-project)

# this should not be changed
set(CMAKE_C_STANDARD 99)

# add your source files
set(SOURCE_FILES src/main.c)

# create a fake library target
add_library(${CMAKE_PROJECT_NAME} ${SOURCE_FILES})

# include naos include paths
add_subdirectory(naos)
`
