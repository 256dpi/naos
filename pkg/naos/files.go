package naos

const mainSourceFile = `#include <naos.h>

static naos_config_t config = {0};

void app_main() {
  // run naos
  naos_init(&config);
  naos_start();
}
`

const projectCMakeListsFile = `# minimal required cmake version
cmake_minimum_required(VERSION 3.10)

# you can set your own project name
project(my-device)

# this should not be changed
set(CMAKE_C_STANDARD 17)

# add your source files
set(SOURCE_FILES src/main.c)

# create a fake library target
add_library(${CMAKE_PROJECT_NAME} ${SOURCE_FILES})

# include naos include paths
add_subdirectory(naos)
`
