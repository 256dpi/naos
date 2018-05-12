PROJECT_NAME := naos-project

export IDF_PATH := ./esp-idf
include $(IDF_PATH)/make/project.mk

define write_component_include
echo "$(1)" >> includes.list;
endef

generate_component_includes:
	rm -f includes.list
	$(foreach x, $(COMPONENT_INCLUDES), $(call write_component_include, $(x)))

update_naos_tree:
	echo "1.22.0-80-g6c4433a-5.2.0" > toolchain.version
	cd esp-idf; git fetch; git checkout v3.0; git submodule update --recursive; cd ..
	cd components/esp-mqtt; git fetch; git checkout v0.6.0; git submodule update --recursive; cd ../..
	cp components/naos/test/sdkconfig sdkconfig
