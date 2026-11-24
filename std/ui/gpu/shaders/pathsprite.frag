#version 450

layout(set = 0, binding = 1) uniform sampler2D rasterised;

layout(location = 0) out vec4 color;

void main() {
    color = texelFetch(rasterised, ivec2(gl_FragCoord.xy), 0);
}
