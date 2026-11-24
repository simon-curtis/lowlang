struct PathSprite {
    vec4 bounds;
};

layout(std430, set = 0, binding = 0) readonly buffer PathSprites {
    PathSprite sprites[];
};
