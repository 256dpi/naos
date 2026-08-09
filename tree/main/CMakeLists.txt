file(GLOB srcs ./src/*.c ./src/*.cpp)
set_property(DIRECTORY APPEND PROPERTY CMAKE_CONFIGURE_DEPENDS ${CMAKE_CURRENT_LIST_DIR}/files.list)
file(STRINGS ${CMAKE_CURRENT_LIST_DIR}/files.list files)
idf_component_register(SRCS ${srcs} INCLUDE_DIRS "" EMBED_TXTFILES ${files})
