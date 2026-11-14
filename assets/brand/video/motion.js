import * as THREE from "three";

export function clamp(x) {
  return Math.min(1, Math.max(0, x));
}

export function smoothstep(from, to, x) {
  const u = clamp((x - from) / (to - from));
  return u * u * (3 - 2 * u);
}

export function easeInOut(x) {
  const u = clamp(x);
  return u < 0.5 ? 4 * u * u * u : 1 - (-2 * u + 2) ** 3 / 2;
}

export function easeOut(x) {
  return 1 - (1 - clamp(x)) ** 3;
}

export function spring(x) {
  if (x <= 0) {
    return 0;
  }
  return 1 - Math.exp(-7 * x) * Math.cos(11 * x);
}

export function between(t, from, to, fade) {
  return smoothstep(from, from + fade, t) * (1 - smoothstep(to - fade, to, t));
}

export function random(seed) {
  let state = seed >>> 0;
  return () => {
    state = (state + 0x6d2b79f5) >>> 0;
    let x = state;
    x = Math.imul(x ^ (x >>> 15), x | 1);
    x ^= x + Math.imul(x ^ (x >>> 7), x | 61);
    return ((x ^ (x >>> 14)) >>> 0) / 4294967296;
  };
}

export function hermite(keys, t, pick) {
  if (t <= keys[0].t) {
    return pick(keys[0]).clone();
  }
  const last = keys.length - 1;
  if (t >= keys[last].t) {
    return pick(keys[last]).clone();
  }
  let k = 0;
  while (keys[k + 1].t < t) {
    k++;
  }
  const tangent = (index) => {
    if (index === 0) {
      return new THREE.Vector3();
    }
    const before = keys[index - 1];
    const after = keys[Math.min(index + 1, last)];
    return pick(after).clone().sub(pick(before)).divideScalar(after.t - before.t);
  };
  const span = keys[k + 1].t - keys[k].t;
  const u = (t - keys[k].t) / span;
  const u2 = u * u;
  const u3 = u2 * u;
  const start = pick(keys[k]).clone().multiplyScalar(2 * u3 - 3 * u2 + 1);
  const leaving = tangent(k).multiplyScalar((u3 - 2 * u2 + u) * span);
  const end = pick(keys[k + 1]).clone().multiplyScalar(-2 * u3 + 3 * u2);
  const arriving = tangent(k + 1).multiplyScalar((u3 - u2) * span);
  return start.add(leaving).add(end).add(arriving);
}
