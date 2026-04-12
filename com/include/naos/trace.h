#ifndef NAOS_TRACE_H
#define NAOS_TRACE_H

#include <stdint.h>

/**
 * NAOS TRACE SUBSYSTEM
 * ====================
 *
 * The trace subsystem provides real-time FreeRTOS task activity recording. It
 * hooks into FreeRTOS via the traceTASK_SWITCHED_IN macro to record context
 * switches, and provides APIs for application-level span and event tracing.
 *
 * Events are written to a circular byte buffer and streamed to clients via the
 * message endpoint (0x08). The buffer uses variable-length records with zero
 * byte padding at wrap boundaries. Records are never split across the buffer
 * end. If the buffer is full, new events are dropped and counted.
 *
 * The record types and their wire format are:
 * - SWITCH (1): TYPE(1) TS(4) CORE(1) ID(1) = 7 bytes
 * - EVENT  (2): TYPE(1) TS(4) CAT(1) NAME(1) ARG(2) = 9 bytes
 * - BEGIN  (3): TYPE(1) TS(4) CAT(1) NAME(1) ARG(2) ID(1) = 10 bytes
 * - END    (4): TYPE(1) TS(4) ID(1) = 6 bytes
 * - VALUE  (5): TYPE(1) TS(4) CAT(1) NAME(1) VAL(4) = 11 bytes
 * - LABEL  (6): TYPE(1) ID(1) TEXT(*) NUL(1) = 3+ bytes
 * - TASK   (7): TYPE(1) ID(1) NAME(*) NUL(1) = 3+ bytes
 *
 * Timestamps are microseconds since trace start (uint32, ~71min overflow).
 * Labels map string pointers to IDs and are written on first use. EVENT and
 * BEGIN reference two label IDs (category and name) and carry a user argument.
 * BEGIN also carries a span instance ID that is matched by END. VALUE records
 * a named counter/gauge as a signed int32. Task names are written inline in
 * TASK records. The read command streams raw buffer records.
 *
 * Endpoint commands:
 * > START (0): - => ACK
 * > STOP  (1): - => ACK
 * > READ  (2): - => DATA(*) + ACK
 * > STATUS(3): - => ACTIVE(1) BUF_SIZE(4) BUF_USED(4) DROPPED(4)
 */

/**
 * Install the trace endpoint. Call once during device setup.
 */
void naos_trace_install();

/**
 * Record an instant trace event. Both strings are used for label dedup
 * (by pointer identity) and must be static.
 *
 * @param category Event category (static string).
 * @param name Event name (static string).
 * @param arg User-defined argument (0-65535).
 */
void naos_trace_event(const char *category, const char *name, uint16_t arg);

/**
 * Record a named value (counter/gauge). Rendered as a track counter in
 * Perfetto. Both strings are used for label dedup (by pointer identity)
 * and must be static.
 *
 * @param category Value category (static string).
 * @param name Value name (static string).
 * @param value Signed 32-bit value.
 */
void naos_trace_value(const char *category, const char *name, int32_t value);

/**
 * Begin a trace span. The returned handle must be passed to naos_trace_end().
 * Both strings are used for label dedup (by pointer identity) and must be
 * static. Returns -1 if tracing is inactive or the label table is full.
 *
 * @param category Span category (static string).
 * @param name Span name (static string).
 * @param arg User-defined argument (0-65535).
 * @return Span handle for naos_trace_end(), or -1.
 */
int naos_trace_begin(const char *category, const char *name, uint16_t arg);

/**
 * End a trace span.
 *
 * @param id Span handle returned by naos_trace_begin(). Negative values are ignored.
 */
void naos_trace_end(int id);

#endif  // NAOS_TRACE_H
