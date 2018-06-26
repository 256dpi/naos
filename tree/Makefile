PROJECT_NAME := naos-project

export IDF_PATH := ./esp-idf
include $(IDF_PATH)/make/project.mk

define write_component_include
echo "$(1)" >> includes.list;
endef

generate_component_includes:
	rm -f includes.list
	$(foreach x, $(COMPONENT_INCLUDES), $(call write_component_include, $(x)))
