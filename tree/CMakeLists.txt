cmake_minimum_required(VERSION 3.5)

if(DEFINED ENV{ADF_PATH})
    include($ENV{ADF_PATH}/CMakeLists.txt)
endif()

include($ENV{IDF_PATH}/tools/cmake/project.cmake)

project(naos-project)
