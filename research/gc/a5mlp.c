#define _GNU_SOURCE
#include <pthread.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <time.h>

#define LINES (16u << 20)
static uint64_t STEPS = 4000000u;

typedef struct { uint64_t next; uint64_t pad[7]; } line_t;

static line_t *buf;
static int chains;

static double now(void) {
    struct timespec t;
    clock_gettime(CLOCK_MONOTONIC, &t);
    return t.tv_sec + t.tv_nsec * 1e-9;
}

static uint64_t rng(uint64_t *s) {
    *s ^= *s << 13; *s ^= *s >> 7; *s ^= *s << 17;
    return *s;
}

static void *chase(void *arg) {
    uint64_t seed = (uintptr_t)arg * 0x9E3779B97F4A7C15ull + 1;
    uint64_t pos[32];
    for (int c = 0; c < chains; c++) pos[c] = rng(&seed) % LINES;
    uint64_t steps = STEPS / chains;
    for (uint64_t i = 0; i < steps; i++)
        for (int c = 0; c < chains; c++) pos[c] = buf[pos[c]].next;
    uint64_t sum = 0;
    for (int c = 0; c < chains; c++) sum += pos[c];
    return (void *)(uintptr_t)sum;
}

static void *stream(void *arg) {
    uint64_t k = (uintptr_t)arg, n = LINES / 12, sum = 0;
    for (uint64_t i = k * n; i < (k + 1) * n; i++) sum += buf[i].next;
    return (void *)(uintptr_t)sum;
}

int main(int argc, char **argv) {
    int huge = argc > 1 && argv[1][0] == 'h';
    buf = mmap(0, sizeof(line_t) * LINES, PROT_READ | PROT_WRITE, MAP_PRIVATE | MAP_ANONYMOUS, -1, 0);
    madvise(buf, sizeof(line_t) * LINES, huge ? MADV_HUGEPAGE : MADV_NOHUGEPAGE);
    uint32_t *perm = malloc(4ull * LINES);
    for (uint32_t i = 0; i < LINES; i++) perm[i] = i;
    uint64_t s = 88172645463325252ull;
    for (uint32_t i = LINES - 1; i > 0; i--) {
        uint32_t j = rng(&s) % i;
        uint32_t t = perm[i]; perm[i] = perm[j]; perm[j] = t;
    }
    for (uint32_t i = 0; i < LINES; i++) buf[perm[i]].next = perm[(i + 1) % LINES];
    free(perm);
    int threads[] = {1, 2, 4, 6, 12};
    int ms[] = {1, 2, 4, 8, 16, 32};
    unsigned nt = sizeof threads / sizeof *threads, nm = sizeof ms / sizeof *ms;
    if (argc > 3) {
        threads[0] = atoi(argv[2]); ms[0] = atoi(argv[3]); nt = nm = 1;
        if (ms[0] == 0) return 0;
        STEPS = 40000000u;
        double t0 = now();
        while (now() - t0 < 0.5) {}
    }
    printf("threads chains Mlines_per_s GB_per_s\n");
    for (unsigned ti = 0; ti < nt; ti++)
        for (unsigned mi = 0; mi < nm; mi++) {
            int k = threads[ti];
            chains = ms[mi];
            pthread_t th[12];
            double t0 = now();
            for (int i = 0; i < k; i++) pthread_create(&th[i], 0, chase, (void *)(uintptr_t)(i + 1));
            for (int i = 0; i < k; i++) pthread_join(th[i], 0);
            double dt = now() - t0;
            double lines = (double)k * (STEPS / chains) * chains;
            printf("%d %d %.1f %.2f\n", k, chains, lines / dt / 1e6, lines * 64 / dt / 1e9);
        }
    pthread_t th[12];
    double t0 = now();
    for (int i = 0; i < 12; i++) pthread_create(&th[i], 0, stream, (void *)(uintptr_t)i);
    for (int i = 0; i < 12; i++) pthread_join(th[i], 0);
    double dt = now() - t0;
    printf("stream12 %.1f %.2f\n", LINES / dt / 1e6, LINES * 64.0 / dt / 1e9);
    return 0;
}
