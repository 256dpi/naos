cmake_minimum_required(VERSION 3.5)

# configure sdkconfig defaults and overrides
set(SDKCONFIG_DEFAULTS sdkconfig.defaults sdkconfig.overrides)

# include esp-adf build system if it exists
if(DEFINED ENV{ADF_PATH})
    include($ENV{ADF_PATH}/CMakeLists.txt)
endif()

# load esp-idf build system
include($ENV{IDF_PATH}/tools/cmake/project.cmake)

# define default project name
set(PROJECT_NAME "naos-project")

# read project name from file if it exists
set(PROJECT_NAME_FILE "${CMAKE_SOURCE_DIR}/project-name.txt")
set_property(DIRECTORY APPEND PROPERTY CMAKE_CONFIGURE_DEPENDS "${PROJECT_NAME_FILE}")
if(EXISTS "${PROJECT_NAME_FILE}")
    file(READ "${PROJECT_NAME_FILE}" PROJECT_NAME_FROM_FILE)
    string(STRIP "${PROJECT_NAME_FROM_FILE}" PROJECT_NAME_FROM_FILE)
    if(NOT "${PROJECT_NAME_FROM_FILE}" STREQUAL "")
        set(PROJECT_NAME "${PROJECT_NAME_FROM_FILE}")
    endif()
endif()

# set project name
project(${PROJECT_NAME})
