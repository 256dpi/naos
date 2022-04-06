file(GLOB srcs ./src/*.c)
file(STRINGS ${CMAKE_CURRENT_LIST_DIR}/files.list files)
idf_component_register(SRCS ${srcs} INCLUDE_DIRS "" EMBED_FILES ${files})
