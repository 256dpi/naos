file(GLOB srcs ./src/*.c)

idf_component_register(SRCS ${srcs}
    INCLUDE_DIRS "include"
    REQUIRES bt esp-mqtt app_update
)
