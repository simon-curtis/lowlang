#version 450
#extension GL_GOOGLE_include_directive : require
#include "common.glsl"
#include "sprite.glsl"

layout(set = 0, binding = 1) uniform sampler2D atlas;

layout(location = 0) flat in int sprite_index;
layout(location = 1) in vec2 pixel;
layout(location = 2) in vec2 uv;
layout(location = 0) out vec4 color;

const float enhanced_contrast = 1.0;
const vec4 gamma_ratios = vec4(0.1469, -0.8911, 1.4644, -0.3234) / 4.0;

float brightness(vec3 rgb) {
    return dot(rgb, vec3(0.30, 0.59, 0.11));
}

float corrected(float coverage, vec3 rgb) {
    float light = brightness(rgb);
    float k = enhanced_contrast * clamp(4.0 * (0.75 - light), 0.0, 1.0);
    float a = coverage * (k + 1.0) / (coverage * k + 1.0);
    float shift = (gamma_ratios.x * light + gamma_ratios.y) * a + (gamma_ratios.z * light + gamma_ratios.w);
    return clamp(a + a * (1.0 - a) * shift, 0.0, 1.0);
}

void main() {
    Sprite sprite = sprites[sprite_index];
    float coverage = corrected(texture(atlas, uv).r, sprite.color.rgb);
    color = vec4(sprite.color.rgb, sprite.color.a * coverage * clip_alpha(pixel, sprite.clip));
}
