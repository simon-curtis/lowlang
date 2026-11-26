#version 450
#extension GL_GOOGLE_include_directive : require
#include "common.glsl"
#include "sprite.glsl"
#include "text.glsl"

layout(set = 0, binding = 1) uniform sampler2D atlas;

layout(location = 0) flat in int sprite_index;
layout(location = 1) in vec2 pixel;
layout(location = 2) in vec2 uv;
layout(location = 0) out vec4 color;

void main() {
    Sprite sprite = sprites[sprite_index];
    float sampled = textureLod(atlas, uv, 0.0).r;
    float coverage = apply_contrast_and_gamma_correction(sampled, sprite.color.rgb, viewport.grayscale_enhanced_contrast, viewport.gamma_ratios);
    color = vec4(sprite.color.rgb, sprite.color.a * coverage * clip_alpha(pixel, sprite.clip));
}
