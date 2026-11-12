struct Quad {
    vec4 bounds;
    vec4 clip;
    vec4 background;
    vec4 border_color;
    vec4 corner_radii;
    vec4 border_widths;
};

layout(std430, set = 0, binding = 0) readonly buffer Quads {
    Quad quads[];
};
