#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ft2build.h>
#include FT_FREETYPE_H
#include FT_OUTLINE_H

static unsigned long long fold(unsigned long long h, long long v) {
    return (h ^ (unsigned long long)v) * 1099511628211ULL;
}

static const unsigned long long offset = 14695981039346656037ULL;

static int load_flags(const char *mode) {
    if (strcmp(mode, "unhinted") == 0) return FT_LOAD_NO_HINTING;
    if (strcmp(mode, "light") == 0) return FT_LOAD_TARGET_LIGHT;
    return -1;
}

static unsigned long long glyph(FT_Face face, FT_UInt g, int flags, int phase, unsigned long long h, FILE *detail, const char *prefix) {
    FT_GlyphSlot slot = face->glyph;
    if (FT_Load_Glyph(face, g, flags)) {
        if (detail) fprintf(detail, "%s %u failed\n", prefix, g);
        return fold(h, -1);
    }
    if (slot->format == FT_GLYPH_FORMAT_OUTLINE) FT_Outline_Translate(&slot->outline, phase * 16, 0);
    if (FT_Render_Glyph(slot, FT_RENDER_MODE_NORMAL)) {
        if (detail) fprintf(detail, "%s %u failed\n", prefix, g);
        return fold(h, -1);
    }
    FT_Bitmap *b = &slot->bitmap;
    unsigned long long own = offset;
    long long head[4] = { b->width, b->rows, slot->bitmap_left, -slot->bitmap_top };
    for (int i = 0; i < 4; i++) {
        h = fold(h, head[i]);
        own = fold(own, head[i]);
    }
    for (unsigned r = 0; r < b->rows; r++) {
        for (unsigned c = 0; c < b->width; c++) {
            unsigned char v = b->buffer[(long)r * b->pitch + c];
            h = fold(h, v);
            own = fold(own, v);
        }
    }
    if (detail) fprintf(detail, "%s %u %u %u %d %d %016llx\n", prefix, g, b->width, b->rows, slot->bitmap_left, -slot->bitmap_top, own);
    return h;
}

int main(int argc, char **argv) {
    FILE *detail = NULL;
    int at = 1;
    if (argc > 1 && strcmp(argv[1], "-glyphs") == 0) {
        detail = stdout;
        at = 2;
    }
    if (argc < at + 4 || (argc - at - 1) % 3 != 0) {
        fprintf(stderr, "usage: ftgold-probe [-glyphs] <font> (<mode> <size 26.6> <phase>)...\n");
        return 2;
    }
    FT_Library lib;
    if (FT_Init_FreeType(&lib)) {
        fprintf(stderr, "cannot initialise FreeType\n");
        return 1;
    }
    FT_Face face;
    if (FT_New_Face(lib, argv[at], 0, &face)) {
        fprintf(stderr, "cannot open %s\n", argv[at]);
        return 1;
    }
    FT_Int major, minor, patch;
    FT_Library_Version(lib, &major, &minor, &patch);
    printf("freetype %d.%d.%d\n", major, minor, patch);
    for (int i = at + 1; i + 2 < argc; i += 3) {
        const char *mode = argv[i];
        int flags = load_flags(mode);
        long size = strtol(argv[i + 1], NULL, 10);
        int phase = (int)strtol(argv[i + 2], NULL, 10);
        if (flags < 0 || size <= 0 || phase < 0 || phase > 3) {
            fprintf(stderr, "bad case %s %s %s\n", argv[i], argv[i + 1], argv[i + 2]);
            return 2;
        }
        if (FT_Set_Char_Size(face, size, size, 0, 0)) {
            fprintf(stderr, "cannot set size %ld\n", size);
            return 1;
        }
        char prefix[64];
        snprintf(prefix, sizeof prefix, "%s %ld %d", mode, size, phase);
        unsigned long long h = offset;
        for (FT_UInt g = 0; g < (FT_UInt)face->num_glyphs; g++) {
            h = glyph(face, g, flags, phase, h, detail, prefix);
        }
        if (!detail) printf("%s %016llx\n", prefix, h);
    }
    FT_Done_Face(face);
    FT_Done_FreeType(lib);
    return 0;
}
