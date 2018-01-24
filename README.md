# naos-tree

**The [Networked Artifacts Operating System](https://github.com/shiftr-io/naos) build tree.**

## Usage

1. Download xtensa toolkit into the directory.
2. Link the source directory to the `main/src/` directory.
3. Run `make` and other commands in the build tree.

## Update

- Set `esp-idf` version: `cd esp-idf; git fetch; git checkout v2.1.1; git submodule update --recursive; cd ..`
- Set `esp-mqtt` version: `cd components/esp-mqtt; git fetch; git checkout v0.4.3; git submodule update --recursive; cd ../..`
- Set `naos-esp` version: `cd components/naos-esp; git fetch; git checkout v0.1.0; git submodule update --recursive; cd ../..`
- Set toolchain version in `toolchain.version`.
