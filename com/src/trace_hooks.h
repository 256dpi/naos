#ifndef NAOS_TRACE_HOOKS_H
#define NAOS_TRACE_HOOKS_H

/*
 * FreeRTOS trace hooks for the naos trace subsystem.
 *
 * This header is force-included via compiler flags (-include) so the
 * trace macros are defined when the FreeRTOS kernel itself is compiled.
 *
 * Do NOT include this header manually.
 */

#ifndef __ASSEMBLER__

extern void naos_trace_task_switched_in(void *task);

#undef traceTASK_SWITCHED_IN
#define traceTASK_SWITCHED_IN() naos_trace_task_switched_in((void *)pxCurrentTCBs[xPortGetCoreID()])

#endif /* __ASSEMBLER__ */

#endif /* NAOS_TRACE_HOOKS_H */
