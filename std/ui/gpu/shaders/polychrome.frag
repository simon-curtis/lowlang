#version 450
#extension GL_GOOGLE_include_directive : require
#include "common.glsl"
#include "polychrome.glsl"

layout(set = 0, binding = 1) uniform sampler2D atlas;

layout(location = 0) flat in int sprite_index;
layout(location = 1) in vec2 pixel;
layout(location = 2) in vec2 uv;
layout(location = 0) out vec4 color;

const vec3 grayscale_factors = vec3(0.2126, 0.7152, 0.0722);

vec4 sample_level(vec4 tile, float level) {
    vec2 half_texel = 0.5 * exp2(level) / vec2(textureSize(atlas, 0));
    vec2 centre = tile.xy + 0.5 * tile.zw;
    vec2 low = min(tile.xy + half_texel, centre);
    vec2 high = max(tile.xy + tile.zw - half_texel, centre);
    return textureLod(atlas, clamp(uv, low, high), level);
}

void main() {
    PolychromeSprite sprite = polychrome_sprites[sprite_index];
    float lower = floor(sprite.effects.z);
    float blend = sprite.effects.z - lower;
    vec4 sampled = sample_level(sprite.tile, lower);
    if (blend > 0.0) {
        sampled = mix(sampled, sample_level(sprite.tile, lower + 1.0), blend);
    }
    float distance = quad_sdf(gl_FragCoord.xy, sprite.bounds, sprite.corner_radii);
    vec4 shaded = sampled;
    if (sprite.effects.x != 0.0) {
        shaded = vec4(vec3(dot(sampled.rgb, grayscale_factors)), sampled.a);
    }
    shaded.a *= sprite.effects.y * clamp(0.5 - distance, 0.0, 1.0) * clip_alpha(pixel, sprite.clip);
    color = shaded;
}
