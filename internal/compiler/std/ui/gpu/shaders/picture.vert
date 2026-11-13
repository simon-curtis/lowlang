#version 450
#extension GL_GOOGLE_include_directive : require
#include "common.glsl"
#include "picture.glsl"

layout(location = 0) flat out int picture_index;
layout(location = 1) out vec2 pixel;
layout(location = 2) out vec2 uv;

void main() {
    Picture picture = pictures[gl_InstanceIndex];
    vec2 corner = unit_corner(gl_VertexIndex);
    pixel = picture.bounds.xy + corner * picture.bounds.zw;
    uv = picture.tile.xy + corner * picture.tile.zw;
    picture_index = gl_InstanceIndex;
    gl_Position = to_device(pixel);
}
