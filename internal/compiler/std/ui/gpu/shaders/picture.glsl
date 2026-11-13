struct Picture {
    vec4 bounds;
    vec4 clip;
    vec4 corner_radii;
    vec4 tile;
    vec4 tone;
};

layout(std430, set = 0, binding = 0) readonly buffer Pictures {
    Picture pictures[];
};
