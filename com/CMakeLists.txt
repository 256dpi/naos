set(srcs
    "src/ble.c"
    "src/coredump.c"
    "src/manager.c"
    "src/monitor.c"
    "src/mqtt.c"
    "src/naos.c"
    "src/net.c"
    "src/params.c"
    "src/settings.c"
    "src/system.c"
    "src/task.c"
    "src/update.c"
    "src/utils.c"
)

idf_component_register(SRCS ${srcs}
    INCLUDE_DIRS "include"
    REQUIRES bt esp-mqtt app_update
)
