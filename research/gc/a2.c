#define _GNU_SOURCE
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

typedef uint64_t u64;

typedef struct {
    u64 count, hs, nw, nb, post;
    u64 *heap, *sb, *mb, *s1, *s2;
} Dump;

static int bit(const u64 *b, u64 i) { return (b[i >> 6] >> (i & 63)) & 1; }
static void setbit(u64 *b, u64 i) { b[i >> 6] |= 1ull << (i & 63); }
static double now(void) {
    struct timespec t;
    clock_gettime(CLOCK_MONOTONIC, &t);
    return t.tv_sec * 1e9 + t.tv_nsec;
}

static int load(FILE *f, Dump *d) {
    u64 h[3];
    if (fread(h, 8, 3, f) != 3) return 0;
    d->post = h[0] >> 63;
    d->count = h[0] & ~(1ull << 63); d->hs = h[1]; d->nw = h[2]; d->nb = (h[2] + 63) / 64;
    d->heap = malloc(8 * d->nw + 8);
    if (d->post) {
        d->sb = d->mb = d->s1 = d->s2 = NULL;
        return fread(d->heap, 8, d->nw, f) == d->nw;
    }
    d->sb = calloc(d->nb + 1, 8);
    d->mb = calloc(d->nb + 1, 8);
    if (fread(d->heap, 8, d->nw, f) != d->nw) return 0;
    if (fread(d->sb, 8, d->nb, f) != d->nb) return 0;
    if (fread(d->mb, 8, d->nb, f) != d->nb) return 0;
    u64 n1 = (d->nb + 63) / 64, n2 = (n1 + 63) / 64;
    d->s1 = calloc(n1 + 1, 8);
    d->s2 = calloc(n2 + 1, 8);
    for (u64 i = 0; i < d->nb; i++) if (d->sb[i]) setbit(d->s1, i);
    for (u64 i = 0; i < n1; i++) if (d->s1[i]) setbit(d->s2, i);
    return 1;
}

static void drop(Dump *d) { free(d->heap); free(d->sb); free(d->mb); free(d->s1); free(d->s2); }

static long prev_set(const u64 *b, u64 i) {
    u64 wi = i >> 6;
    u64 m = b[wi] & ((2ull << (i & 63)) - 1);
    if (m) return (long)(wi * 64 + 63 - __builtin_clzll(m));
    return -1;
}

static long resolve(const Dump *d, u64 w) {
    if (w < d->hs + 8 || w >= d->hs + 8 * d->nw + 8) return -1;
    u64 i = (w - 8 - d->hs) >> 3;
    long r = prev_set(d->sb, i);
    if (r >= 0) return r;
    u64 wi = i >> 6;
    if (wi == 0) return -1;
    long a = prev_set(d->s1, wi - 1);
    if (a < 0) {
        u64 w1 = (wi - 1) >> 6;
        if (w1 == 0) return -1;
        long b = prev_set(d->s2, w1 - 1);
        while (b < 0) {
            u64 w2 = (w1 - 1) >> 6;
            if (w2 == 0) return -1;
            w1 = w2 * 64;
            b = prev_set(d->s2, w1 - 1);
        }
        a = 64 * b + 63 - __builtin_clzll(d->s1[b]);
    }
    return a * 64 + 63 - __builtin_clzll(d->sb[a]);
}

static u64 size_of(const Dump *d, u64 i) { return d->heap[i] >> 3; }
static int is_free(const Dump *d, u64 i) { return (d->heap[i] & 2) != 0; }
static int is_leaf(const Dump *d, u64 i) { return (d->heap[i] & 4) != 0; }

static u64 *stack;
static u64 stack_cap;

static u64 mark(const Dump *d, u64 *out, const u64 *roots, u64 nroots) {
    u64 sp = 0, live = 0;
    for (u64 r = 0; r < nroots; r++) {
        u64 i = roots[r];
        if (bit(out, i)) continue;
        setbit(out, i);
        live += size_of(d, i);
        if (!is_leaf(d, i)) stack[sp++] = i;
        while (sp) {
            u64 o = stack[--sp];
            u64 n = size_of(d, o);
            for (u64 k = 1; k < n; k++) {
                long t = resolve(d, d->heap[o + k]);
                if (t < 0 || bit(out, t) || is_free(d, t)) continue;
                setbit(out, t);
                live += size_of(d, t);
                if (!is_leaf(d, t)) {
                    if (sp == stack_cap) { stack_cap *= 2; stack = realloc(stack, 8 * stack_cap); }
                    stack[sp++] = t;
                }
            }
        }
    }
    return live;
}

static u64 *roots_for(const Dump *d, u64 *nroots) {
    u64 *ref = calloc(d->nb + 1, 8);
    for (u64 wi = 0; wi < d->nb; wi++)
        for (u64 m = d->mb[wi]; m; m &= m - 1) {
            u64 o = wi * 64 + __builtin_ctzll(m);
            if (is_leaf(d, o)) continue;
            for (u64 k = 1; k < size_of(d, o); k++) {
                long t = resolve(d, d->heap[o + k]);
                if (t >= 0 && t != (long)o) setbit(ref, t);
            }
        }
    u64 n = 0, *r = malloc(8 * (d->nw / 2 + 1));
    for (u64 wi = 0; wi < d->nb; wi++)
        for (u64 m = d->mb[wi] & ~ref[wi]; m; m &= m - 1) r[n++] = wi * 64 + __builtin_ctzll(m);
    for (u64 wi = 0; wi < d->nb; wi++)
        for (u64 m = d->mb[wi] & ref[wi]; m; m &= m - 1) r[n++] = wi * 64 + __builtin_ctzll(m);
    free(ref);
    *nroots = n;
    return r;
}

#define P1 0x9E3779B185EBCA87ull
#define P2 0xC2B2AE3D27D4EB4Full
static u64 rotl(u64 x, int r) { return (x << r) | (x >> (64 - r)); }

static u64 obj_hash(const u64 *w, u64 n, int ptr_only, u64 lo, u64 hi) {
    u64 a = P1, b = P2, c = 0, e = P1 ^ P2;
    u64 k = 0;
    if (!ptr_only) {
        for (; k + 4 <= n; k += 4) {
            a = rotl(a + w[k] * P2, 31) * P1;
            b = rotl(b + w[k + 1] * P2, 31) * P1;
            c = rotl(c + w[k + 2] * P2, 31) * P1;
            e = rotl(e + w[k + 3] * P2, 31) * P1;
        }
        for (; k < n; k++) a = rotl(a + w[k] * P2, 31) * P1;
    } else {
        for (; k < n; k++) {
            u64 v = w[k] - lo < hi - lo ? w[k] : 0;
            a = rotl(a + v * P2, 31) * P1;
        }
    }
    return rotl(a, 1) + rotl(b, 7) + rotl(c, 12) + rotl(e, 18) + n;
}

static u64 hash_pages(const Dump *d, const u64 *set, u64 nb, u64 *ph, int ptr_only) {
    u64 words = 0, lo = d->hs, hi = d->hs + 85899345920ull;
    memset(ph, 0, 8 * (d->nw / 512 + 2));
    for (u64 wi = 0; wi < nb; wi++)
        for (u64 m = set[wi]; m; m &= m - 1) {
            u64 o = wi * 64 + __builtin_ctzll(m);
            if (is_leaf(d, o)) continue;
            u64 n = size_of(d, o);
            words += n;
            ph[o >> 9] ^= obj_hash(d->heap + o, n, ptr_only, lo, hi) * ((o << 1) | 1);
        }
    return words;
}

static u64 __attribute__((unused)) hash_runs(const Dump *d, const u64 *set, u64 nb, u64 *ph) {
    u64 words = 0, rs = 0, re = 0;
    int open = 0;
    memset(ph, 0, 8 * (d->nw / 512 + 2));
    for (u64 wi = 0; wi < nb; wi++)
        for (u64 m = set[wi]; m; m &= m - 1) {
            u64 o = wi * 64 + __builtin_ctzll(m);
            if (is_leaf(d, o)) continue;
            u64 n = size_of(d, o);
            words += n;
            if (open && o == re && (o >> 9) == (rs >> 9)) { re = o + n; continue; }
            if (open) ph[rs >> 9] ^= obj_hash(d->heap + rs, re - rs, 0, 0, 0) * ((rs << 1) | 1);
            rs = o; re = o + n; open = 1;
        }
    if (open) ph[rs >> 9] ^= obj_hash(d->heap + rs, re - rs, 0, 0, 0) * ((rs << 1) | 1);
    return words;
}

static unsigned char *full_old;

static void mark_full_old(const Dump *d) {
    u64 np = d->nw / 512 + 2;
    free(full_old);
    full_old = calloc(np, 1);
    for (u64 p = 0; p * 512 + 512 <= d->nw; p++) {
        int same = 1, any = 0;
        for (u64 k = 0; k < 8; k++) {
            same &= d->sb[p * 8 + k] == d->mb[p * 8 + k];
            any |= d->mb[p * 8 + k] != 0;
        }
        int scan = 0;
        for (u64 k = 0; k < 8 && same && any && !scan; k++)
            for (u64 m = d->mb[p * 8 + k]; m && !scan; m &= m - 1)
                scan = !is_leaf(d, (p * 8 + k) * 64 + __builtin_ctzll(m));
        full_old[p] = same && any && scan;
    }
}

static u64 hash_hybrid(const Dump *d, const u64 *set, u64 nb, u64 *ph) {
    u64 words = 0, rs = 0, re = 0;
    int open = 0;
    memset(ph, 0, 8 * (d->nw / 512 + 2));
    for (u64 wi = 0; wi < nb; wi++) {
        u64 p = wi >> 3;
        if (full_old[p]) {
            if ((wi & 7) == 0) {
                u64 end = p * 512 + 512;
                for (int k = 7; k >= 0; k--) {
                    u64 b = d->sb[p * 8 + k];
                    if (!b) continue;
                    u64 last = (p * 8 + k) * 64 + 63 - __builtin_clzll(b);
                    if (!is_leaf(d, last) && last + size_of(d, last) > end) end = last + size_of(d, last);
                    break;
                }
                ph[p] = obj_hash(d->heap + p * 512, end - p * 512, 0, 0, 0);
                words += end - p * 512;
            }
            continue;
        }
        for (u64 m = set[wi]; m; m &= m - 1) {
            u64 o = wi * 64 + __builtin_ctzll(m);
            if (is_leaf(d, o)) continue;
            u64 n = size_of(d, o);
            words += n;
            if (open && o == re && (o >> 9) == (rs >> 9)) { re = o + n; continue; }
            if (open) ph[rs >> 9] ^= obj_hash(d->heap + rs, re - rs, 0, 0, 0) * ((rs << 1) | 1);
            rs = o; re = o + n; open = 1;
        }
    }
    if (open) ph[rs >> 9] ^= obj_hash(d->heap + rs, re - rs, 0, 0, 0) * ((rs << 1) | 1);
    return words;
}

static void hash_raw(const Dump *d, u64 *ph) {
    for (u64 p = 0; p * 512 < d->nw; p++) {
        u64 n = d->nw - p * 512 < 512 ? d->nw - p * 512 : 512;
        ph[p] = obj_hash(d->heap + p * 512, n, 0, 0, 0);
    }
}

static u64 scan_old(const Dump *d, const u64 *old, u64 nb, unsigned char *dirty) {
    u64 hits = 0;
    for (u64 wi = 0; wi < nb; wi++)
        for (u64 m = old[wi]; m; m &= m - 1) {
            u64 o = wi * 64 + __builtin_ctzll(m);
            if (is_leaf(d, o)) continue;
            u64 n = size_of(d, o);
            for (u64 k = 1; k < n; k++) {
                long t = resolve(d, d->heap[o + k]);
                if (t < 0) continue;
                if ((u64)t >> 6 < nb && bit(old, t)) continue;
                if (is_free(d, t)) continue;
                dirty[o >> 9] = 1;
                hits++;
            }
        }
    return hits;
}

static unsigned char *young_pages(const Dump *d, const u64 *old, u64 onb) {
    unsigned char *yp = calloc(d->nw / 512 + 2, 1);
    for (u64 wi = 0; wi < d->nb; wi++)
        for (u64 m = d->sb[wi] & ~(wi < onb ? old[wi] : 0); m; m &= m - 1) {
            u64 o = wi * 64 + __builtin_ctzll(m);
            if (is_free(d, o)) continue;
            u64 e = o + (size_of(d, o) ? size_of(d, o) : 1) - 1;
            for (u64 q = o >> 9; q <= e >> 9; q++) yp[q] = 1;
        }
    return yp;
}

static u64 scan2(const Dump *d, const u64 *old, u64 nb, const unsigned char *yp, unsigned char *dirty) {
    u64 hits = 0, lo = d->hs + 8, span = 8 * d->nw;
#define CHECK(w, page) do { u64 v_ = (w); \
        if (v_ - lo < span && yp[(v_ - lo) >> 12]) { \
            long t_ = resolve(d, v_); \
            if (t_ >= 0 && !((u64)t_ >> 6 < nb && bit(old, t_)) && !is_free(d, t_)) { dirty[page] = 1; hits++; } \
        } } while (0)
    for (u64 wi = 0; wi < nb; wi++) {
        u64 p = wi >> 3;
        if (full_old[p]) {
            if ((wi & 7) == 0) {
                u64 end = p * 512 + 512;
                for (int k = 7; k >= 0; k--) {
                    u64 b = d->sb[p * 8 + k];
                    if (!b) continue;
                    u64 last = (p * 8 + k) * 64 + 63 - __builtin_clzll(b);
                    if (!is_leaf(d, last) && last + size_of(d, last) > end) end = last + size_of(d, last);
                    break;
                }
                if (end > d->nw) end = d->nw;
                for (u64 k = p * 512; k < end; k++) CHECK(d->heap[k], p);
            }
            continue;
        }
        for (u64 m = old[wi]; m; m &= m - 1) {
            u64 o = wi * 64 + __builtin_ctzll(m);
            if (is_leaf(d, o)) continue;
            u64 n = size_of(d, o);
            for (u64 k = 1; k < n; k++) CHECK(d->heap[o + k], o >> 9);
        }
    }
    return hits;
}

int main(int argc, char **argv) {
    FILE *f = fopen(argv[1], "rb");
    int reps = argc > 2 ? atoi(argv[2]) : 3;
    stack_cap = 1 << 20;
    stack = malloc(8 * stack_cap);
    Dump prev = {0}, cur;
    u64 *prev_h = NULL, *prev_h2 = NULL, *prev_raw = NULL;
    int have_prev = 0, post_seen = 0;
    printf("gc live_kb old_scan_kb young_kb mark_ns_kb hash_ns_kb scan_ns_kb hash_vs_mark scan_vs_hash old_pages dirty_h dirty_h2 dirty_s dirty_raw unsound proj_minor_vs_full mark_match hash_ns live_w dirty_w young_w scan2_vs_hash dirty_s2 raw_post\n");
    while (load(f, &cur)) {
        if (cur.post) {
            if (have_prev && cur.count == prev.count && prev_raw) {
                u64 pn = prev.nw / 512 + 2;
                free(prev_raw);
                prev_raw = calloc(pn > cur.nw / 512 + 2 ? pn : cur.nw / 512 + 2, 8);
                hash_raw(&cur, prev_raw);
                post_seen = 1;
            }
            free(cur.heap);
            continue;
        }
        u64 np = cur.nw / 512 + 2;
        u64 *h = calloc(np, 8), *h2 = calloc(np, 8), *raw = calloc(np, 8);
        hash_raw(&cur, raw);
        u64 nroots;
        u64 *roots = roots_for(&cur, &nroots);
        double tm = 1e30, th = 1e30, ts = 1e30;
        u64 *out = calloc(cur.nb + 1, 8), live = 0;
        for (int r = 0; r < reps; r++) {
            memset(out, 0, 8 * (cur.nb + 1));
            double t0 = now();
            live = mark(&cur, out, roots, nroots);
            double t1 = now();
            if (t1 - t0 < tm) tm = t1 - t0;
        }
        u64 mism = 0;
        for (u64 wi = 0; wi < cur.nb; wi++) mism += __builtin_popcountll(out[wi] ^ cur.mb[wi]);
        int pair = have_prev && cur.count == prev.count + 1 && cur.hs == prev.hs;
        if (pair) {
            u64 onb = prev.nb < cur.nb ? prev.nb : cur.nb;
            u64 oldw = 0;
            for (int r = 0; r < reps; r++) {
                double t0 = now();
                oldw = hash_hybrid(&cur, prev.mb, onb, h);
                double t1 = now();
                if (t1 - t0 < th) th = t1 - t0;
            }
            hash_pages(&cur, prev.mb, onb, h2, 1);
            unsigned char *ds = calloc(np, 1);
            for (int r = 0; r < reps; r++) {
                memset(ds, 0, np);
                double t0 = now();
                scan_old(&cur, prev.mb, onb, ds);
                double t1 = now();
                if (t1 - t0 < ts) ts = t1 - t0;
            }
            unsigned char *yp = young_pages(&cur, prev.mb, onb);
            unsigned char *ds2 = calloc(np, 1);
            double ts2 = 1e30;
            for (int r = 0; r < reps; r++) {
                memset(ds2, 0, np);
                double t0 = now();
                scan2(&cur, prev.mb, onb, yp, ds2);
                double t1 = now();
                if (t1 - t0 < ts2) ts2 = t1 - t0;
            }
            u64 ds2n = 0;
            u64 pnp = prev.nw / 512 + 2, opages = 0, dh = 0, dh2 = 0, draw = 0, dsn = 0, unsound = 0, dirty_words = 0;
            unsigned char *haso = calloc(np, 1);
            for (u64 wi = 0; wi < onb; wi++)
                for (u64 m = prev.mb[wi]; m; m &= m - 1) {
                    u64 o = wi * 64 + __builtin_ctzll(m);
                    if (!is_leaf(&cur, o)) haso[o >> 9] = 1;
                }
            for (u64 p = 0; p < np && p < pnp; p++) {
                if (!haso[p]) continue;
                opages++;
                int d1 = h[p] != prev_h[p], d2 = h2[p] != prev_h2[p];
                dh += d1; dh2 += d2; draw += raw[p] != prev_raw[p]; dsn += ds[p]; ds2n += ds2[p];
                if (ds[p] && !d1) unsound++;
            }
            for (u64 wi = 0; wi < onb; wi++)
                for (u64 m = prev.mb[wi]; m; m &= m - 1) {
                    u64 o = wi * 64 + __builtin_ctzll(m);
                    if (!is_leaf(&cur, o) && h[o >> 9] != prev_h[o >> 9]) dirty_words += size_of(&cur, o);
                }
            u64 young = 0;
            for (u64 wi = 0; wi < cur.nb; wi++)
                for (u64 m = cur.mb[wi] & ~(wi < onb ? prev.mb[wi] : 0); m; m &= m - 1)
                    young += size_of(&cur, wi * 64 + __builtin_ctzll(m));
            double per_word = tm / live;
            double minor = th + per_word * (dirty_words + young);
            printf("%llu %llu %llu %llu %.1f %.1f %.1f %.3f %.3f %llu %.4f %.4f %.4f %.4f %llu %.3f %llu %.0f %llu %llu %llu %.3f %.4f %d\n",
                   (unsigned long long)cur.count, (unsigned long long)(live / 128), (unsigned long long)(oldw / 128),
                   (unsigned long long)(young / 128), tm / (live / 128.0), th / (oldw / 128.0 + 1e-9),
                   ts / (oldw / 128.0 + 1e-9), th / tm, ts / th, (unsigned long long)opages,
                   (double)dh / (opages + 1e-9), (double)dh2 / (opages + 1e-9), (double)dsn / (opages + 1e-9), (double)draw / (opages + 1e-9),
                   (unsigned long long)unsound, minor / tm, (unsigned long long)mism, th,
                   (unsigned long long)live, (unsigned long long)dirty_words, (unsigned long long)young,
                   ts2 / th, (double)ds2n / (opages + 1e-9), post_seen);
            free(ds); free(haso); free(yp); free(ds2);
        }
        u64 onb2 = cur.nb;
        mark_full_old(&cur);
        hash_hybrid(&cur, cur.mb, onb2, h);
        hash_pages(&cur, cur.mb, onb2, h2, 1);
        free(prev_h); free(prev_h2); free(prev_raw);
        post_seen = 0;
        prev_raw = raw;
        prev_h = h; prev_h2 = h2;
        if (have_prev) drop(&prev);
        prev = cur;
        have_prev = 1;
        free(roots); free(out);
    }
    return 0;
}
