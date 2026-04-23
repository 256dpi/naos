#ifndef NAOS_TIME_H
#define NAOS_TIME_H

/**
 * NAOS TIME SUBSYSTEM
 * ===================
 *
 * Provides timekeeping on top of POSIX: a background SNTP client and a
 * persisted IANA timezone name. Reading, setting, and formatting time
 * remain POSIX (`time`, `clock_gettime`, `settimeofday`, `localtime_r`,
 * `gmtime_r`, `strftime`).
 *
 * Timezone:
 *   - `time-tz-name` (persisted, user-settable) is authoritative. Set it
 *     to an IANA zone like "Europe/Zurich" or "America/New_York".
 *   - `time-tz-posix` (volatile, locked) is the derived POSIX TZ string
 *     produced by looking up the name in the bundled IANA tzdata table
 *     (see tools/tzdata.py / src/tzdata.c). Empty if the name is unknown.
 *   - On lookup miss the system falls back to UTC0 and logs a warning.
 *   - Flash cost of the bundled table is ~15 KB for ~540 zones.
 *
 * SNTP:
 *   - `time-sntp-list` (persisted) is a comma-separated server list.
 *     An empty list disables SNTP.
 *   - The SNTP client only runs while the device is at least
 *     NAOS_CONNECTED. It starts/stops automatically as connectivity
 *     changes and as the list is edited.
 *   - `time-sntp-status` (volatile, locked) reflects the current state:
 *       "disabled"  — list is empty, SNTP not running.
 *       "offline"   — list is non-empty but SNTP is not running because
 *                     the device is not connected.
 *       "unsynced"  — running, no successful sync yet.
 *       "synced"    — most recent poll succeeded.
 *       "failed"    — grace period elapsed without a successful poll.
 *
 * Usage:
 *   - Detecting clock changes: cache a time_t from gettimeofday() and
 *     re-read on a timer; treat any step larger than a couple of seconds
 *     as a clock set. SNTP and the endpoint `set` command step the clock
 *     (smooth_sync is disabled), so a simple diff is reliable.
 *   - Syncing with an external RTC/GPS: at boot, read the hardware once
 *     and call settimeofday() with the value. To write the clock back to
 *     the RTC after SNTP or a manual set, use the detection pattern above
 *     and write on a detected step.
 *   - Applying a manual UI time change: call settimeofday() with the new
 *     UTC value. No notification API is needed — other components pick
 *     up the change via the detection pattern. Note that SNTP will
 *     overwrite the manual value on its next successful poll unless it
 *     is disabled by clearing `time-sntp-list`.
 */

/**
 * Initializes the subsystem.
 */
void naos_time_init(void);

#endif  // NAOS_TIME_H
