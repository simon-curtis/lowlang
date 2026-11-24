#version 450
#extension GL_GOOGLE_include_directive : require
#include "common.glsl"
#include "pathsprite.glsl"

void main() {
    PathSprite sprite = sprites[gl_InstanceIndex];
    gl_Position = to_device(sprite.bounds.xy + unit_corner(gl_VertexIndex) * sprite.bounds.zw);
}
