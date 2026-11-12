struct Shadow {
    vec4 bounds;
    vec4 clip;
    vec4 color;
    vec4 corner_radii;
    vec4 blur;
};

layout(std430, set = 0, binding = 0) readonly buffer Shadows {
    Shadow shadows[];
};
