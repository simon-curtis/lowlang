float color_brightness(vec3 color) {
    return dot(color, vec3(0.30, 0.59, 0.11));
}

float light_on_dark_contrast(float enhanced_contrast, vec3 color) {
    float brightness = color_brightness(color);
    float multiplier = clamp(4.0 * (0.75 - brightness), 0.0, 1.0);
    return enhanced_contrast * multiplier;
}

float enhance_contrast(float alpha, float k) {
    return alpha * (k + 1.0) / (alpha * k + 1.0);
}

vec3 enhance_contrast3(vec3 alpha, float k) {
    return alpha * (k + 1.0) / (alpha * k + 1.0);
}

float apply_alpha_correction(float a, float b, vec4 g) {
    float brightness_adjustment = g.x * b + g.y;
    float correction = brightness_adjustment * a + (g.z * b + g.w);
    return a + a * (1.0 - a) * correction;
}

vec3 apply_alpha_correction3(vec3 a, vec3 b, vec4 g) {
    vec3 brightness_adjustment = g.x * b + g.y;
    vec3 correction = brightness_adjustment * a + (g.z * b + g.w);
    return a + a * (1.0 - a) * correction;
}

float apply_contrast_and_gamma_correction(float sample_value, vec3 color, float enhanced_contrast_factor, vec4 gamma_ratios) {
    float enhanced_contrast = light_on_dark_contrast(enhanced_contrast_factor, color);
    float brightness = color_brightness(color);
    float contrasted = enhance_contrast(sample_value, enhanced_contrast);
    return apply_alpha_correction(contrasted, brightness, gamma_ratios);
}

vec3 apply_contrast_and_gamma_correction3(vec3 sample_value, vec3 color, float enhanced_contrast_factor, vec4 gamma_ratios) {
    float enhanced_contrast = light_on_dark_contrast(enhanced_contrast_factor, color);
    vec3 contrasted = enhance_contrast3(sample_value, enhanced_contrast);
    return apply_alpha_correction3(contrasted, color, gamma_ratios);
}
