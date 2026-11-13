#version 450
#extension GL_GOOGLE_include_directive : require
#include "common.glsl"
#include "picture.glsl"

layout(set = 0, binding = 2) uniform sampler2D colors;

layout(location = 0) flat in int picture_index;
layout(location = 1) in vec2 pixel;
layout(location = 2) in vec2 uv;
layout(location = 0) out vec4 color;

void main() {
    Picture picture = pictures[picture_index];
    vec2 half_size = picture.bounds.zw * 0.5;
    vec2 center = picture.bounds.xy + half_size;
    float radius = pick_corner(pixel - center, picture.corner_radii);
    float coverage = clamp(0.5 - rounded_distance(pixel, center, half_size, radius), 0.0, 1.0);
    vec4 sampled = texture(colors, uv);
    float luminance = dot(sampled.rgb, vec3(0.2126, 0.7152, 0.0722));
    vec3 rgb = mix(sampled.rgb, vec3(luminance), picture.tone.y);
    color = vec4(rgb, sampled.a * picture.tone.x * coverage * clip_alpha(pixel, picture.clip));
}
