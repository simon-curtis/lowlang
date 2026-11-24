#version 450

layout(location = 0) in vec2 st;
layout(location = 1) flat in vec4 color;
layout(location = 2) in vec4 clip_distance;
layout(location = 0) out vec4 premultiplied;

void main() {
    vec2 dx = dFdx(st);
    vec2 dy = dFdy(st);
    float alpha;
    if (length(vec2(dx.x, dy.x)) < 0.001) {
        alpha = 1.0;
    } else {
        vec2 gradient = 2.0 * st.xx * vec2(dx.x, dy.x) - vec2(dx.y, dy.y);
        float f = st.x * st.x - st.y;
        alpha = clamp(0.5 - f / length(gradient), 0.0, 1.0);
    }
    if (any(lessThan(clip_distance, vec4(0.0)))) {
        alpha = 0.0;
    }
    premultiplied = vec4(color.rgb * color.a * alpha, color.a * alpha);
}
