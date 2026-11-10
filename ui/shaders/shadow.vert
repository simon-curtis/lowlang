#version 450
#extension GL_GOOGLE_include_directive : require
#include "common.glsl"
#include "shadow.glsl"

layout(location = 0) flat out int shadow_index;
layout(location = 1) out vec2 pixel;

void main() {
    Shadow shadow = shadows[gl_InstanceIndex];
    float margin = 3.0 * shadow.blur.x;
    vec2 origin = shadow.bounds.xy - margin;
    vec2 size = shadow.bounds.zw + 2.0 * margin;
    pixel = origin + unit_corner(gl_VertexIndex) * size;
    shadow_index = gl_InstanceIndex;
    gl_Position = to_device(pixel);
}
