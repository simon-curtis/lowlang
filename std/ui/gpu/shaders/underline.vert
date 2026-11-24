#version 450
#extension GL_GOOGLE_include_directive : require
#include "common.glsl"
#include "underline.glsl"

layout(location = 0) flat out int underline_index;
layout(location = 1) out vec2 pixel;

void main() {
    Underline underline = underlines[gl_InstanceIndex];
    pixel = underline.bounds.xy + unit_corner(gl_VertexIndex) * underline.bounds.zw;
    underline_index = gl_InstanceIndex;
    gl_Position = to_device(pixel);
}
