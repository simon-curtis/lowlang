#version 450
#extension GL_GOOGLE_include_directive : require
#include "common.glsl"
#include "quad.glsl"

layout(location = 0) flat out int quad_index;
layout(location = 1) out vec2 pixel;

void main() {
    Quad quad = quads[gl_InstanceIndex];
    pixel = quad.bounds.xy + unit_corner(gl_VertexIndex) * quad.bounds.zw;
    quad_index = gl_InstanceIndex;
    gl_Position = to_device(pixel);
}
