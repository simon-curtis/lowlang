layout(push_constant) uniform Viewport {
    vec2 size;
} viewport;

vec2 unit_corner(int index) {
    const vec2 corners[6] = vec2[6](
        vec2(0.0, 0.0), vec2(1.0, 0.0), vec2(0.0, 1.0),
        vec2(1.0, 0.0), vec2(1.0, 1.0), vec2(0.0, 1.0)
    );
    return corners[index];
}

vec4 to_device(vec2 position) {
    return vec4(position / viewport.size * 2.0 - 1.0, 0.0, 1.0);
}

float pick_corner(vec2 offset, vec4 radii) {
    if (offset.x < 0.0) {
        return offset.y < 0.0 ? radii.x : radii.w;
    }
    return offset.y < 0.0 ? radii.y : radii.z;
}

float rounded_distance(vec2 point, vec2 center, vec2 half_size, float radius) {
    vec2 corner = abs(point - center) - half_size + radius;
    return length(max(corner, 0.0)) + min(max(corner.x, corner.y), 0.0) - radius;
}

float clip_alpha(vec2 point, vec4 clip) {
    vec2 inside = step(clip.xy, point) * step(point, clip.xy + clip.zw);
    return inside.x * inside.y;
}
