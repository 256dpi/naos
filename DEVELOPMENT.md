# Development

## SDK Versions

The versions of the various SDK used are managed in the `tree` directory. If the version files are updated the git modules needs to be updated with `make tree_update`. Additionally, the test components needs to be updated with `make update`.

## Partition Table & SDK Config

Changes to the partition table and SDK config need to be synced from the tree to the component using `make sync`. This ensures that the proper values are set at the source where the naos cli will look when building a project.

## Release

Tag release on GitHub. The naos cli will checkout the repo and automatically update all dependencies.
