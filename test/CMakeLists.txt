cmake_minimum_required(VERSION 3.7)
project(naos-project)

set(CMAKE_C_STANDARD 99)

set(SOURCE_FILES src/main.c)

add_library(naos-project ${SOURCE_FILES})

add_subdirectory(.naos)
