file(GLOB srcs ./src/*.c)
include(files.cmake)
idf_component_register(SRCS ${srcs} INCLUDE_DIRS "" EMBED_FILES ${files})
