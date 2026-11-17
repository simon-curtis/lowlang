#version 450
#extension GL_GOOGLE_include_directive : require
#include "common.glsl"
#include "sprite.glsl"
#include "text.glsl"

layout(set = 0, binding = 1) uniform sampler2D atlas;

layout(location = 0) flat in int sprite_index;
layout(location = 1) in vec2 pixel;
layout(location = 2) in vec2 uv;
layout(location = 0, index = 0) out vec4 foreground;
layout(location = 0, index = 1) out vec4 stripe_coverage;

void main() {
    Sprite sprite = sprites[sprite_index];
    vec3 sampled = texture(atlas, uv).rgb;
    if (viewport.is_bgr != 0u) {
        sampled = sampled.bgr;
    }
    vec3 corrected = apply_contrast_and_gamma_correction3(sampled, sprite.color.rgb, viewport.subpixel_enhanced_contrast, viewport.gamma_ratios);
    foreground = vec4(sprite.color.rgb, 1.0);
    stripe_coverage = vec4(sprite.color.a * corrected * clip_alpha(pixel, sprite.clip), 1.0);
}
