#version 450
#extension GL_GOOGLE_include_directive : require
#include "common.glsl"
#include "quad.glsl"

layout(location = 0) flat in int quad_index;
layout(location = 1) in vec2 pixel;
layout(location = 0) out vec4 color;

const float antialias_threshold = 0.5;

vec4 over(vec4 below, vec4 above) {
    float alpha = above.a + below.a * (1.0 - above.a);
    vec3 rgb = (above.rgb * above.a + below.rgb * below.a * (1.0 - above.a)) / alpha;
    return vec4(rgb, alpha);
}

float quarter_ellipse_sdf(vec2 point, vec2 radii) {
    vec2 circle_vec = point / radii;
    float unit_circle_sdf = length(circle_vec) - 1.0;
    return unit_circle_sdf * (radii.x + radii.y) * -0.5;
}

vec4 shade(Quad quad, vec2 position) {
    vec4 background_color = quad.background;
    vec4 radii = quad.corner_radii;
    vec4 widths = quad.border_widths;
    bool unrounded = radii.x == 0.0 && radii.y == 0.0 && radii.z == 0.0 && radii.w == 0.0;

    if (widths.x == 0.0 && widths.y == 0.0 && widths.z == 0.0 && widths.w == 0.0 && unrounded) {
        return background_color;
    }

    vec2 half_size = quad.bounds.zw / 2.0;
    vec2 the_point = position - quad.bounds.xy;
    vec2 center_to_point = the_point - half_size;
    float corner_radius = pick_corner(center_to_point, radii);

    vec2 border = vec2(
        center_to_point.x < 0.0 ? widths.w : widths.y,
        center_to_point.y < 0.0 ? widths.x : widths.z
    );
    vec2 reduced_border = vec2(
        border.x == 0.0 ? -antialias_threshold : border.x,
        border.y == 0.0 ? -antialias_threshold : border.y
    );

    vec2 corner_to_point = abs(center_to_point) - half_size;
    vec2 corner_center_to_point = corner_to_point + corner_radius;
    bool is_near_rounded_corner = corner_center_to_point.x >= 0.0 && corner_center_to_point.y >= 0.0;
    vec2 straight_border_inner_corner_to_point = corner_to_point + reduced_border;
    bool is_beyond_inner_straight_border =
        straight_border_inner_corner_to_point.x > 0.0 || straight_border_inner_corner_to_point.y > 0.0;
    bool is_within_inner_straight_border =
        straight_border_inner_corner_to_point.x < -antialias_threshold &&
        straight_border_inner_corner_to_point.y < -antialias_threshold;

    if (is_within_inner_straight_border && !is_near_rounded_corner) {
        return background_color;
    }

    float outer_sdf = quad_sdf_impl(corner_center_to_point, corner_radius);

    float inner_sdf = 0.0;
    if (corner_center_to_point.x <= 0.0 || corner_center_to_point.y <= 0.0) {
        inner_sdf = -max(straight_border_inner_corner_to_point.x, straight_border_inner_corner_to_point.y);
    } else if (is_beyond_inner_straight_border) {
        inner_sdf = -1.0;
    } else if (reduced_border.x == reduced_border.y) {
        inner_sdf = -(outer_sdf + reduced_border.x);
    } else {
        vec2 ellipse_radii = max(vec2(0.0), vec2(corner_radius) - reduced_border);
        inner_sdf = quarter_ellipse_sdf(corner_center_to_point, ellipse_radii);
    }

    float border_sdf = max(inner_sdf, outer_sdf);

    vec4 shaded = background_color;
    if (border_sdf < antialias_threshold) {
        vec4 blended_border = over(background_color, quad.border_color);
        shaded = mix(background_color, blended_border, clamp(antialias_threshold - inner_sdf, 0.0, 1.0));
    }
    return shaded * vec4(1.0, 1.0, 1.0, clamp(antialias_threshold - outer_sdf, 0.0, 1.0));
}

void main() {
    Quad quad = quads[quad_index];
    vec4 shaded = shade(quad, gl_FragCoord.xy);
    color = vec4(shaded.rgb, shaded.a * clip_alpha(pixel, quad.clip));
}
