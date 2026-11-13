struct Sprite {
    vec4 bounds;
    vec4 clip;
    vec4 color;
    vec4 tile;
};

layout(std430, set = 0, binding = 0) readonly buffer Sprites {
    Sprite sprites[];
};
