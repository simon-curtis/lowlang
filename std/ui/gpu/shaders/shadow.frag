#version 450
#extension GL_GOOGLE_include_directive : require
#include "common.glsl"
#include "shadow.glsl"

layout(location = 0) flat in int shadow_index;
layout(location = 1) in vec2 pixel;
layout(location = 0) out vec4 color;

const float PI = 3.141592653589793;

float gaussian(float x, float sigma) {
    return exp(-(x * x) / (2.0 * sigma * sigma)) / (sqrt(2.0 * PI) * sigma);
}

vec2 erf(vec2 x) {
    vec2 s = sign(x);
    vec2 a = abs(x);
    x = 1.0 + (0.278393 + (0.230389 + 0.078108 * (a * a)) * a) * a;
    x *= x;
    return s - s / (x * x);
}

float blur_along_x(float x, float y, float sigma, float corner, vec2 half_size) {
    float delta = min(half_size.y - corner - abs(y), 0.0);
    float curved = half_size.x - corner + sqrt(max(0.0, corner * corner - delta * delta));
    vec2 integral = 0.5 + 0.5 * erf((x + vec2(-curved, curved)) * (sqrt(0.5) / sigma));
    return integral.y - integral.x;
}

void main() {
    Shadow shadow = shadows[shadow_index];
    float blur_radius = shadow.blur.x;
    vec2 half_size = shadow.bounds.zw / 2.0;
    vec2 center = shadow.bounds.xy + half_size;
    vec2 point0 = gl_FragCoord.xy - center;
    float corner_radius = pick_corner(point0, shadow.corner_radii);

    float alpha;
    if (blur_radius == 0.0) {
        float distance = quad_sdf(gl_FragCoord.xy, shadow.bounds, shadow.corner_radii);
        alpha = clamp(0.5 - distance, 0.0, 1.0);
    } else {
        float low = point0.y - half_size.y;
        float high = point0.y + half_size.y;
        float start = clamp(-3.0 * blur_radius, low, high);
        float end = clamp(3.0 * blur_radius, low, high);
        float step_size = (end - start) / 4.0;
        float y = start + step_size * 0.5;
        alpha = 0.0;
        for (int i = 0; i < 4; i++) {
            alpha += blur_along_x(point0.x, point0.y - y, blur_radius, corner_radius, half_size) * gaussian(y, blur_radius) * step_size;
            y += step_size;
        }
    }
    color = vec4(shadow.color.rgb, shadow.color.a * alpha * clip_alpha(pixel, shadow.clip));
}
