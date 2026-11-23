#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

typedef uint64_t u64;

static int bit(const u64 *b, u64 i) { return (b[i >> 6] >> (i & 63)) & 1; }

static u64 rng = 88172645463325252ull;
static u64 next(void) { rng ^= rng << 13; rng ^= rng >> 7; rng ^= rng << 17; return rng; }

int main(int argc, char **argv) {
    FILE *f = fopen(argv[1], "rb");
    int probes = argc > 2 ? atoi(argv[2]) : 64;
    printf("gc live_mb live_pages ideal_pages sparse50 mesh_pairs mesh_saved_frac occupancy_gap_frac\n");
    u64 h[3];
    while (fread(h, 8, 3, f) == 3) {
        int post = h[0] >> 63;
        u64 nw = h[2], nb = (nw + 63) / 64;
        u64 *heap = malloc(8 * nw + 8);
        if (fread(heap, 8, nw, f) != nw) break;
        if (post) { free(heap); continue; }
        u64 *sb = malloc(8 * nb), *mb = malloc(8 * nb);
        if (fread(sb, 8, nb, f) != nb || fread(mb, 8, nb, f) != nb) break;
        u64 np = (nw + 511) / 512;
        u64 *occ = calloc(np * 8, 8);
        u64 live = 0;
        for (u64 wi = 0; wi < nb; wi++)
            for (u64 m = mb[wi]; m; m &= m - 1) {
                u64 o = wi * 64 + __builtin_ctzll(m);
                u64 n = heap[o] >> 3;
                if (!n) n = 1;
                live += n;
                for (u64 k = o; k < o + n && k < nw; ) {
                    if ((k & 63) == 0 && k + 64 <= o + n) { occ[k >> 6] = ~0ull; k += 64; continue; }
                    occ[k >> 6] |= 1ull << (k & 63);
                    k++;
                }
            }
        u64 live_pages = 0, sparse = 0;
        u64 *cand = malloc(8 * np);
        int *cnt = calloc(np, sizeof(int));
        for (u64 p = 0; p < np; p++) {
            int c = 0;
            for (int k = 0; k < 8; k++) c += __builtin_popcountll(occ[p * 8 + k]);
            cnt[p] = c;
            if (c) live_pages++;
            if (c && c <= 256) cand[sparse++] = p;
        }
        unsigned char *used = calloc(np, 1);
        u64 pairs = 0;
        for (u64 i = 0; i < sparse; i++) {
            u64 a = cand[i];
            if (used[a]) continue;
            for (int t = 0; t < probes; t++) {
                u64 b = cand[next() % sparse];
                if (b == a || used[b]) continue;
                int clash = 0;
                for (int k = 0; k < 8 && !clash; k++) clash = (occ[a * 8 + k] & occ[b * 8 + k]) != 0;
                if (clash) continue;
                used[a] = used[b] = 1;
                pairs++;
                break;
            }
        }
        u64 ideal = (live + 511) / 512;
        printf("%llu %.1f %llu %llu %llu %llu %.3f %.3f\n", (unsigned long long)h[0], live / 131072.0,
               (unsigned long long)live_pages, (unsigned long long)ideal, (unsigned long long)sparse,
               (unsigned long long)pairs, (double)pairs / (live_pages + 1e-9),
               1.0 - (double)ideal / (live_pages + 1e-9));
        free(heap); free(sb); free(mb); free(occ); free(cand); free(cnt); free(used);
    }
    return 0;
}
