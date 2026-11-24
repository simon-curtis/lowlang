struct Underline {
    vec4 bounds;
    vec4 clip;
    vec4 color;
    vec4 stroke;
};

layout(std430, set = 0, binding = 0) readonly buffer Underlines {
    Underline underlines[];
};
