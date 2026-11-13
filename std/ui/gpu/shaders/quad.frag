#version 450
#extension GL_GOOGLE_include_directive : require
#include "common.glsl"
#include "quad.glsl"

layout(location = 0) flat in int quad_index;
layout(location = 1) in vec2 pixel;
layout(location = 0) out vec4 color;

void main() {
    Quad quad = quads[quad_index];
    vec2 half_size = quad.bounds.zw * 0.5;
    vec2 center = quad.bounds.xy + half_size;
    vec2 offset = pixel - center;
    float radius = pick_corner(offset, quad.corner_radii);
    float outer = rounded_distance(pixel, center, half_size, radius);

    vec4 widths = quad.border_widths;
    vec2 inset_origin = quad.bounds.xy + vec2(widths.w, widths.x);
    vec2 inset_size = max(quad.bounds.zw - vec2(widths.y + widths.w, widths.x + widths.z), vec2(0.0));
    vec2 inset_half = inset_size * 0.5;
    float inner_radius = max(radius - max(max(widths.x, widths.y), max(widths.z, widths.w)), 0.0);
    float inner = rounded_distance(pixel, inset_origin + inset_half, inset_half, inner_radius);

    float coverage = clamp(0.5 - outer, 0.0, 1.0);
    float border = clamp(0.5 + inner, 0.0, 1.0);
    bool has_border = any(greaterThan(widths, vec4(0.0)));
    vec4 fill = has_border ? mix(quad.background, quad.border_color, border) : quad.background;
    color = vec4(fill.rgb, fill.a * coverage * clip_alpha(pixel, quad.clip));
}
