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
  NAOS_LOCAL, /** Messages are transferred under the configured base topic of the device. */
  NAOS_GLOBAL /** Messages are transferred on a global level. */
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
  NAOS_DISCONNECTED, /** Device is disconnected. */
  NAOS_CONNECTED,    /** Device is connected to a WiFi access point. */
  NAOS_NETWORKED     /** device is networked with a MQTT broker. */
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
  NAOS_STRING,
  NAOS_BOOL,
  NAOS_LONG,
  NAOS_DOUBLE,
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
   * The default values set if the parameter is missing.
   */
  union {
    const char *default_s;
    bool default_b;
    int32_t default_l;
    double default_d;
  };

  /**
   * The synchronized variables.
   */
  union {
    char **sync_s;
    bool *sync_b;
    int32_t *sync_l;
    double *sync_d;
  };

  /**
   * The synchronization functions.
   */
  union {
    void (*func_s)(const char *);
    void (*func_b)(bool);
    void (*func_l)(int32_t);
    void (*func_d)(double);
  };
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
   * The parameters to be registered during initialization.
   */
  naos_param_t *parameters;
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
   * This callback is called to retrieve battery information.
   *
   * @return A value between 0 and 1 indicating the battery charge level.
   */
  float (*battery_callback)();

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
 * Register a parameter.
 */
void naos_register(naos_param_t *param);

/**
 * Will return the value of the requested parameter.
 *
 * @note A returned pointer is only valid until the next call.
 *
 * @param param The parameter.
 * @return Pointer to value.
 */
const char *naos_get(const char *param);
bool naos_get_b(const char *param);
int32_t naos_get_l(const char *param);
double naos_get_d(const char *param);

/**
 * Will set the value of the requested parameter. Synchronized parameters are automatically updated.
 *
 * @param param The parameter.
 * @param value The value.
 */
void naos_set(const char *param, const char *value);
void naos_set_b(const char *param, bool value);
void naos_set_l(const char *param, int32_t value);
void naos_set_d(const char *param, double value);

/**
 * Will lookup the specified parameter.
 * @param name The parameter.
 * @return A reference of NULL if not found.
 */
naos_param_t *naos_lookup(const char *name);

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
 * @param len The payload length.
 * @param qos The QoS level.
 * @param retained The retained flag.
 * @param scope The scope.
 * @return Whether the command was successful.
 */
bool naos_publish(const char *topic, const char *payload, int qos, bool retained, naos_scope_t scope);
bool naos_publish_b(const char *topic, bool payload, int qos, bool retained, naos_scope_t scope);
bool naos_publish_l(const char *topic, int32_t payload, int qos, bool retained, naos_scope_t scope);
bool naos_publish_d(const char *topic, double payload, int qos, bool retained, naos_scope_t scope);
bool naos_publish_r(const char *topic, void *payload, size_t len, int qos, bool retained, naos_scope_t scope);

/**
 * Returns the current status.
 */
naos_status_t naos_status();

/**
 * The message will be printed to the serial port and published to the broker if logging is activated.
 *
 * @param fmt The message format.
 * @param ... The used arguments.
 */
void naos_log(const char *fmt, ...);

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
