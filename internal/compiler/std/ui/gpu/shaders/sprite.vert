#version 450
#extension GL_GOOGLE_include_directive : require
#include "common.glsl"
#include "sprite.glsl"

layout(location = 0) flat out int sprite_index;
layout(location = 1) out vec2 pixel;
layout(location = 2) out vec2 uv;

void main() {
    Sprite sprite = sprites[gl_InstanceIndex];
    vec2 corner = unit_corner(gl_VertexIndex);
    pixel = sprite.bounds.xy + corner * sprite.bounds.zw;
    uv = sprite.tile.xy + corner * sprite.tile.zw;
    sprite_index = gl_InstanceIndex;
    gl_Position = to_device(pixel);
}
