// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0
//
// Minimal HelixQA LD_PRELOAD shim template. Compile with:
//   gcc -shared -fPIC -o shim.so ld-preload-shim.c -ldl
//
// Override any libc function you want to observe. The shim writes
// newline-delimited JSON records to the FIFO named by
// HELIXQA_LD_SHIM_FIFO. HelixQA's ld_preload Observer reads that
// FIFO and emits Events with Kind == "hook".
//
// JSON record format (one object per line, no trailing comma):
//   {"ts_ns":<int64>,"fn":"<function_name>","arg":"<first_arg_string>"}
//
// Operator workflow:
//   1. mkfifo /tmp/helix-shim.fifo
//   2. Compile: gcc -shared -fPIC -o /tmp/shim.so ld-preload-shim.c -ldl
//   3. Set target.Labels["shim_path"] = "/tmp/shim.so" in your bank YAML,
//      OR export HELIXQA_LD_SHIM=/tmp/shim.so
//   4. HelixQA sets LD_PRELOAD and HELIXQA_LD_SHIM_FIFO automatically
//      when launching the target process via productionProducer.Produce.
//   5. Read events from obs.Events() — each carries Kind="hook",
//      Payload["fn"], Payload["arg"], and the nanosecond Timestamp.

#define _GNU_SOURCE
#include <dlfcn.h>
#include <fcntl.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

static FILE *shim_out = NULL;

// ensure_out opens the FIFO on first use.  Thread-safety left to the
// operator for this template; production shims should add a pthread_once.
static void ensure_out(void) {
    if (shim_out) return;
    const char *path = getenv("HELIXQA_LD_SHIM_FIFO");
    if (!path || !*path) return;
    shim_out = fopen(path, "a");
}

// shim_emit writes one JSON record to the FIFO.
// arg may be NULL (emitted as empty string).
static void shim_emit(const char *fn, const char *arg) {
    ensure_out();
    if (!shim_out) return;
    struct timespec ts;
    clock_gettime(CLOCK_REALTIME, &ts);
    long long ts_ns = (long long)ts.tv_sec * 1000000000LL + ts.tv_nsec;
    fprintf(shim_out,
            "{\"ts_ns\":%lld,\"fn\":\"%s\",\"arg\":\"%s\"}\n",
            ts_ns,
            fn  ? fn  : "",
            arg ? arg : "");
    fflush(shim_out);
}

// ---------------------------------------------------------------------------
// Example override: open(2)
//
// Records every file-open attempt with the pathname as the arg.
// O_CREAT mode handling is simplified — production shims should use
// va_arg to forward the mode argument when (flags & O_CREAT).
// ---------------------------------------------------------------------------
int open(const char *pathname, int flags, ...) {
    static int (*real_open)(const char *, int, ...) = NULL;
    if (!real_open) real_open = dlsym(RTLD_NEXT, "open");
    shim_emit("open", pathname);
    if (flags & O_CREAT) {
        va_list ap;
        va_start(ap, flags);
        mode_t mode = va_arg(ap, mode_t);
        va_end(ap);
        return real_open(pathname, flags, mode);
    }
    return real_open(pathname, flags);
}

// ---------------------------------------------------------------------------
// Extend with additional overrides as needed, for example:
//
// ssize_t read(int fd, void *buf, size_t count) { ... }
// ssize_t write(int fd, const void *buf, size_t count) { ... }
// int connect(int sockfd, const struct sockaddr *addr, socklen_t addrlen) { ... }
// int socket(int domain, int type, int protocol) { ... }
// ---------------------------------------------------------------------------
