struct PolychromeSprite {
    vec4 bounds;
    vec4 clip;
    vec4 tile;
    vec4 corner_radii;
    vec4 effects;
};

layout(std430, set = 0, binding = 0) readonly buffer PolychromeSprites {
    PolychromeSprite polychrome_sprites[];
};
