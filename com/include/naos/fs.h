#ifndef NAOS_FS_H
#define NAOS_FS_H

/**
 * Mount a "data/fat" partition as a FAT filesystem.
 *
 * Note: The size of partition should be at least 528kB.
 *
 * @param path The mount path.
 * @param label The partition label.
 * @param max_files The maximum number of open files.
 */
void naos_fs_mount_fat(const char *path, const char *label, int max_files);

typedef struct {
  /**
   * The exposed filesystem root.
   */
  const char *root;

  /**
   * The NULL-terminated list of entries to expose at the root level. If set,
   * listing "/" will return these entries instead of reading the root directory.
   */
  const char **root_entries;
} naos_fs_config_t;

/**
 * Install the FS endpoint.
 */
void naos_fs_install(naos_fs_config_t cfg);

#endif // NAOS_FS_H
