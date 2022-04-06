#ifndef NAOS_H
#define NAOS_H

#include <stdarg.h>
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

/**
 * The messages scopes.
 */
typedef enum {
  /**
   * The local scope denotes messages that are transferred under the configured base topic of the device.
   */
  NAOS_LOCAL,

  /**
   * The global scope denotes messages that are transferred on a global level.
   */
  NAOS_GLOBAL
} naos_scope_t;

/**
 * Get the string representation of the specified scope.
 *
 * @param scope The scope.
 * @return The string value.
 */
const char *naos_scope_str(naos_scope_t scope);

/**
 * The system statuses.
 */
typedef enum {
  /**
   * Device is disconnected.
   */
  NAOS_DISCONNECTED,

  /**
   * The device is connected to a WiFi access point.
   */
  NAOS_CONNECTED,

  /**
   * The device is networked with a MQTT broker.
   */
  NAOS_NETWORKED
} naos_status_t;

/**
 * Get the string representation of the specified status.
 *
 * @param status The status.
 * @return The string value.
 */
const char *naos_status_str(naos_status_t status);

/**
 * The parameter types.
 */
typedef enum {
  /**
   * A string parameter.
   */
  NAOS_STRING,

  /**
   * A boolean parameter.
   */
  NAOS_BOOL,

  /**
   * A long parameter.
   */
  NAOS_LONG,

  /**
   * A double parameter.
   */
  NAOS_DOUBLE
} naos_type_t;

/**
 * A single parameter.
 */
typedef struct {
  /**
   * The name of the parameter e.g. "my-param".
   */
  const char *name;

  /**
   * The parameter type.
   */
  naos_type_t type;

  /**
   * The default string value.
   */
  const char *default_s;

  /**
   * The default boolean value.
   */
  bool default_b;

  /**
   * The default long value.
   */
  int32_t default_l;

  /**
   * The default double value.
   */
  double default_d;

  /**
   * The synchronized string variable.
   */
  char **sync_s;

  /**
   * The synchronized boolean variable.
   */
  bool *sync_b;

  /**
   * The synchronized long variable.
   */
  int32_t *sync_l;

  /**
   * The synchronized double variable.
   */
  double *sync_d;

  /**
   * The string synchronization function.
   */
  void (*func_s)(char *);

  /**
   * The boolean synchronization function.
   */
  void (*func_b)(bool);

  /**
   * The long synchronization function.
   */
  void (*func_l)(int32_t);

  /**
   * The double synchronization function.
   */
  void (*func_d)(double);
} naos_param_t;

/**
 * The main configuration object.
 */
typedef struct {
  /**
   * The device type.
   */
  const char *device_type;

  /**
   * The firmware version.
   */
  const char *firmware_version;

  /**
   * The managed parameters.
   */
  naos_param_t *parameters;

  /**
   * The number of managed parameters.
   */
  size_t num_parameters;

  /**
   * The callback that is called after initialization on the application core (1).
   *
   * Note: The esp-idf `app_main` function is called on the protocol core (0). Therefore, interrupts created from its
   * context are assigned to the protocol core and might conflict with interrupts used by the WiFi and BLE stack.
   */
  void (*setup_callback)();

  /**
   *  The callback that is called when a ping is received.
   */
  void (*ping_callback)();

  /**
   * The callback that is called once the device comes online.
   */
  void (*online_callback)();

  /**
   * The callback that is called when a parameter has been updated. If the parameter was set, value is the string value.
   * If the parameter was unset, value is NULL.
   *
   * @param param The parameter.
   * @param value The value.
   */
  void (*update_callback)(const char *param, const char *value);

  /**
   * The message callback is called with incoming messages.
   *
   * @note The base topic has already been removed from the topic and should not start with a '/'.
   *
   * @param topic The topic.
   * @param payload The payload.
   * @param len The payload length.
   * @param scope The scope.
   */
  void (*message_callback)(const char *topic, uint8_t *payload, size_t len, naos_scope_t scope);

  /**
   * The loop callback is called repeatedly if the device is online.
   */
  void (*loop_callback)();

  /**
   * The interval of the loop callback in milliseconds.
   */
  int loop_interval;

  /**
   * The offline callback is called once the device becomes offline.
   */
  void (*offline_callback)();

  /**
   * The callback is called once the device has changed its status.
   *
   * @param status The status.
   */
  void (*status_callback)(naos_status_t status);

  /**
   * This callback is called to retrieve battery information. It should be provided if the device has a battery power
   * source.
   *
   * @return A value between 0 and 1 indicating the battery charge level.
   */
  float (*battery_level)();

  /**
   * A password to protect Bluetooth access.
   */
  const char *password;

  /**
   * If set, the device will randomly (up to 5s) delay startup to overcome WiFi and MQTT congestion issues if many
   * devices restart at the same time.
   */
  bool delay_startup;
} naos_config_t;

/**
 * Initialize the system.
 *
 * @note Should only be called once on boot.
 *
 * @param config The configuration object.
 */
void naos_init(naos_config_t *config);

/**
 * The message will be printed to the serial port and published to the broker if logging is activated.
 *
 * @param fmt The message format.
 * @param ... The used arguments.
 */
void naos_log(const char *fmt, ...);

/**
 * Will return the value of the requested parameter. It will return an empty string for not set parameters.
 *
 * @note The returned pointer is only valid until the next call.
 *
 * @param param The parameter.
 * @return Pointer to value.
 */
char *naos_get(const char *param);

/**
 * Will return the value of the requested parameter.
 *
 * @param param The parameter.
 * @return The value.
 */
bool naos_get_b(const char *param);

/**
 * Will return the value of the requested parameter.
 *
 * @param param The parameter.
 * @return The value.
 */
int32_t naos_get_l(const char *param);

/**
 * Will return the value of the requested parameter.
 *
 * @param param The parameter.
 * @return The value.
 */
double naos_get_d(const char *param);

/**
 * Will set the value of the requested parameter. Synchronized parameters are automatically updated.
 *
 * @param param The parameter.
 * @param value The value.
 */
void naos_set(const char *param, const char *value);

/**
 * Will set the value of the requested parameter. Synchronized parameters are automatically updated.
 *
 * @param param The parameter.
 * @param value The value.
 */
void naos_set_b(const char *param, bool value);

/**
 * Will set the value of the requested parameter. Synchronized parameters are automatically updated.
 *
 * @param param The parameter.
 * @param value The value.
 */
void naos_set_l(const char *param, int32_t value);

/**
 * Will set the value of the requested parameter. Synchronized parameters are automatically updated.
 *
 * @param param The parameter.
 * @param value The value.
 */
void naos_set_d(const char *param, double value);

/**
 * Ensure a default value of a parameter if it is missing.
 *
 * @param param The parameter.
 * @param value The value.
 * @return Whether the parameter was set.
 */
bool naos_ensure(const char *param, const char *value);

/**
 * Ensure a default value of a parameter if it is missing.
 *
 * @param param The parameter.
 * @param value The value.
 * @return Whether the parameter was set.
 */
bool naos_ensure_b(const char *param, bool value);

/**
 * Ensure a default value of a parameter if it is missing.
 *
 * @param param The parameter.
 * @param value The value.
 * @return Whether the parameter was set.
 */
bool naos_ensure_l(const char *param, int32_t value);

/**
 * Ensure a default value of a parameter if it is missing.
 *
 * @param param The parameter.
 * @param value The value.
 * @return Whether the parameter was set.
 */
bool naos_ensure_d(const char *param, double value);

/**
 * Will unset the requested parameter. Synchronized parameters are automatically updated.
 *
 * @param param The parameter.
 * @return Whether the parameter was unset.
 */
bool naos_unset(const char *param);

/**
 * Will automatically apply parameter modifications and set the specified pointer to a buffer that contains the set
 * value or an empty string if unset. Alternatively, it may just call the provided function with the new value or a
 * combination if both mechanisms. It will read, set and yield the currently stored value when invoked.
 *
 * @note This function should only be called from app_main() to ensure only one synchronization is registered per
 * parameter.
 *
 * @param param The parameter.
 * @param pointer The pointer.
 * @param func The function.
 * @return Whether the registration was successful.
 */
bool naos_sync(const char *param, char **pointer, void (*func)(char *));

/**
 * Will automatically apply parameter modifications and set the specified pointer to a buffer that contains the set
 * value or an empty string if unset. Alternatively, it may just call the provided function with the new value or a
 * combination if both mechanisms. It will read, set and yield the currently stored value when invoked.
 *
 * @note This function should only be called from app_main() to ensure only one synchronization is registered per
 * parameter.
 *
 * @param param The parameter.
 * @param pointer The pointer.
 * @param func The function.
 * @return Whether the registration was successful.
 */
bool naos_sync_b(const char *param, bool *pointer, void (*func)(bool));

/**
 * Will automatically apply parameter modifications and set the specified pointer to a buffer that contains the set
 * value or an empty string if unset. Alternatively, it may just call the provided function with the new value or a
 * combination if both mechanisms. It will read, set and yield the currently stored value when invoked.
 *
 * @note This function should only be called from app_main() to ensure only one synchronization is registered per
 * parameter.
 *
 * @param param The parameter.
 * @param pointer The pointer.
 * @param func The function.
 * @return Whether the registration was successful.
 */
bool naos_sync_l(const char *param, int32_t *pointer, void (*func)(int32_t));

/**
 * Will automatically apply parameter modifications and set the specified pointer to a buffer that contains the set
 * value or an empty string if unset. Alternatively, it may just call the provided function with the new value or a
 * combination if both mechanisms. It will read, set and yield the currently stored value when invoked.
 *
 * @note This function should only be called from app_main() to ensure only one synchronization is registered per
 * parameter.
 *
 * @param param The parameter.
 * @param pointer The pointer.
 * @param func The function.
 * @return Whether the registration was successful.
 */
bool naos_sync_d(const char *param, double *pointer, void (*func)(double));

/**
 * Subscribe to specified topic. The topic is automatically prefixed with the configured base topic if the scope is
 * local.
 *
 * @param topic The topic.
 * @param qos The QoS level.
 * @param scope The scope.
 * @return Whether the command was successful.
 */
bool naos_subscribe(const char *topic, int qos, naos_scope_t scope);

/**
 * Unsubscribe from specified topic. The topic is automatically prefixed with the configured base topic if the scope is
 * local.
 *
 * @param topic The topic.
 * @param scope The scope.
 * @return Whether the command was successful.
 */
bool naos_unsubscribe(const char *topic, naos_scope_t scope);

/**
 * Publish to the specified topic. The topic is automatically prefixed with the configured base topic if the scope is
 * local.
 *
 * @param topic The topic.
 * @param payload The payload.
 * @param qos The QoS level.
 * @param retained The retained flag.
 * @param scope The scope.
 * @return Whether the command was successful.
 */
bool naos_publish(const char *topic, const char *payload, int qos, bool retained, naos_scope_t scope);

/**
 * Publish to the specified topic. The topic is automatically prefixed with the configured base topic if the scope is
 * local.
 *
 * @param topic The topic.
 * @param payload The payload.
 * @param qos The QoS level.
 * @param retained The retained flag.
 * @param scope The scope.
 * @return Whether the command was successful.
 */
bool naos_publish_b(const char *topic, bool payload, int qos, bool retained, naos_scope_t scope);

/**
 * Publish to the specified topic. The topic is automatically prefixed with the configured base topic if the scope is
 * local.
 *
 * @param topic The topic.
 * @param payload The payload.
 * @param qos The QoS level.
 * @param retained The retained flag.
 * @param scope The scope.
 * @return Whether the command was successful.
 */
bool naos_publish_l(const char *topic, int32_t payload, int qos, bool retained, naos_scope_t scope);

/**
 * Publish to the specified topic. The topic is automatically prefixed with the configured base topic if the scope is
 * local.
 *
 * @param topic The topic.
 * @param payload The payload.
 * @param qos The QoS level.
 * @param retained The retained flag.
 * @param scope The scope.
 * @return Whether the command was successful.
 */
bool naos_publish_d(const char *topic, double payload, int qos, bool retained, naos_scope_t scope);

/**
 * Publish to the specified topic. The topic is automatically prefixed with the configured base topic if the scope is
 * local.
 *
 * @param topic The topic.
 * @param payload The payload.
 * @param len The payload length.
 * @param qos The QoS level.
 * @param retained The retained flag.
 * @param scope The scope.
 * @return Whether the command was successful.
 */
bool naos_publish_r(const char *topic, void *payload, size_t len, int qos, bool retained, naos_scope_t scope);

/**
 * Returns the current status.
 */
naos_status_t naos_status();

/**
 * Returns the elapsed milliseconds since the start.
 *
 * @return The elapsed milliseconds.
 */
uint32_t naos_millis();

/**
 * Will delay current task for the specified amount of milliseconds.
 *
 * @note This function should only be used inside the loop callback.
 *
 * @param ms The amount of milliseconds to delay.
 */
void naos_delay(uint32_t ms);

/**
 * Acquire will acquire the global naos mutex. This allows to synchronize custom callbacks with naos callbacks.
 */
void naos_acquire();

/**
 * Release will release the global naos mutex. This allows to synchronize custom callbacks with naos callbacks.
 */
void naos_release();

#endif  // NAOS_H
