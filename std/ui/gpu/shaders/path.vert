#version 450
#extension GL_GOOGLE_include_directive : require
#include "common.glsl"
#include "path.glsl"

layout(location = 0) out vec2 st;
layout(location = 1) flat out vec4 color;
layout(location = 2) out vec4 clip_distance;

void main() {
    PathVertex vertex = vertices[gl_VertexIndex];
    st = vertex.st;
    color = vertex.color;
    clip_distance = vec4(
        vertex.position.x - vertex.clip.x,
        vertex.clip.x + vertex.clip.z - vertex.position.x,
        vertex.position.y - vertex.clip.y,
        vertex.clip.y + vertex.clip.w - vertex.position.y
    );
    gl_Position = to_device(vertex.position);
}
