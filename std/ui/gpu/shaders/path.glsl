struct PathVertex {
    vec2 position;
    vec2 st;
    vec4 color;
    vec4 clip;
};

layout(std430, set = 0, binding = 0) readonly buffer PathVertices {
    PathVertex vertices[];
};
