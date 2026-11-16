layout(push_constant) uniform Viewport {
    vec2 size;
    vec4 gamma_ratios;
    float grayscale_enhanced_contrast;
    float subpixel_enhanced_contrast;
    uint is_bgr;
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

float quad_sdf_impl(vec2 corner_center_to_point, float corner_radius) {
    if (corner_radius == 0.0) {
        return max(corner_center_to_point.x, corner_center_to_point.y);
    }
    float signed_distance_to_inset_quad =
        length(max(vec2(0.0), corner_center_to_point)) +
        min(0.0, max(corner_center_to_point.x, corner_center_to_point.y));
    return signed_distance_to_inset_quad - corner_radius;
}

float quad_sdf(vec2 point, vec4 bounds, vec4 radii) {
    vec2 half_size = bounds.zw / 2.0;
    vec2 center_to_point = point - (bounds.xy + half_size);
    float corner_radius = pick_corner(center_to_point, radii);
    vec2 corner_center_to_point = abs(center_to_point) - half_size + corner_radius;
    return quad_sdf_impl(corner_center_to_point, corner_radius);
}

float clip_alpha(vec2 point, vec4 clip) {
    vec2 inside = step(clip.xy, point) * step(point, clip.xy + clip.zw);
    return inside.x * inside.y;
}
