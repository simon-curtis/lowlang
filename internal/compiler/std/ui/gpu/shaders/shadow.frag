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
    vec2 r1 = 1.0 + (0.278393 + (0.230389 + (0.000972 + 0.078108 * a) * a) * a) * a;
    vec2 r2 = r1 * r1;
    return s - s / (r2 * r2);
}

float blur_along_x(float x, float y, float sigma, float corner, vec2 half_size) {
    float delta = min(half_size.y - corner - abs(y), 0.0);
    float curved = half_size.x - corner + sqrt(max(0.0, corner * corner - delta * delta));
    vec2 integral = 0.5 + 0.5 * erf((x + vec2(-curved, curved)) * (sqrt(0.5) / sigma));
    return integral.y - integral.x;
}

void main() {
    Shadow shadow = shadows[shadow_index];
    vec2 half_size = shadow.bounds.zw * 0.5;
    vec2 center = shadow.bounds.xy + half_size;
    vec2 point = pixel - center;
    float corner = pick_corner(point, shadow.corner_radii);
    float sigma = max(shadow.blur.x, 0.5);
    float low = point.y - half_size.y;
    float high = point.y + half_size.y;
    float start = clamp(-3.0 * sigma, low, high);
    float end = clamp(3.0 * sigma, low, high);
    float step_size = (end - start) / 4.0;
    float y = start + step_size * 0.5;
    float alpha = 0.0;
    for (int i = 0; i < 4; i++) {
        alpha += blur_along_x(point.x, point.y - y, sigma, corner, half_size) * gaussian(y, sigma) * step_size;
        y += step_size;
    }
    color = vec4(shadow.color.rgb, shadow.color.a * alpha * clip_alpha(pixel, shadow.clip));
}
