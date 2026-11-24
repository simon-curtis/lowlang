#version 450
#extension GL_GOOGLE_include_directive : require
#include "common.glsl"
#include "underline.glsl"

layout(location = 0) flat in int underline_index;
layout(location = 1) in vec2 pixel;
layout(location = 0) out vec4 color;

const float PI = 3.141592653;
const float WAVE_FREQUENCY = 2.0;
const float WAVE_HEIGHT_RATIO = 0.8;

float wave_alpha(Underline underline) {
    float thickness = underline.stroke.x;
    float height = underline.bounds.w;
    float half_thickness = thickness * 0.5;
    vec2 st = ((gl_FragCoord.xy - underline.bounds.xy) / height) - vec2(0.0, 0.5);
    float frequency = (PI * WAVE_FREQUENCY * thickness) / height;
    float amplitude = (thickness * WAVE_HEIGHT_RATIO) / height;
    float sine = sin(st.x * frequency) * amplitude;
    float d_sine = cos(st.x * frequency) * amplitude * frequency;
    float distance = (st.y - sine) / sqrt(1.0 + d_sine * d_sine);
    float distance_in_pixels = distance * height;
    float distance_from_top_border = distance_in_pixels - half_thickness;
    float distance_from_bottom_border = distance_in_pixels + half_thickness;
    return clamp(0.5 - max(-distance_from_bottom_border, distance_from_top_border), 0.0, 1.0);
}

void main() {
    Underline underline = underlines[underline_index];
    float alpha = underline.stroke.y != 0.0 ? wave_alpha(underline) : 1.0;
    color = vec4(underline.color.rgb, underline.color.a * alpha * clip_alpha(pixel, underline.clip));
}
