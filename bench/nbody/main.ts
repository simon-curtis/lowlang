interface Body {
  x: number;
  y: number;
  z: number;
  vx: number;
  vy: number;
  vz: number;
  mass: number;
}

function system(): Body[] {
  const solar = 4 * Math.PI * Math.PI;
  const days = 365.24;
  const bodies: Body[] = [
    { x: 0, y: 0, z: 0, vx: 0, vy: 0, vz: 0, mass: solar },
    {
      x: 4.8414314424647209,
      y: -1.16032004402742839,
      z: -1.03622044471123109e-1,
      vx: 1.66007664274403694e-3 * days,
      vy: 7.69901118419740425e-3 * days,
      vz: -6.90460016972063023e-5 * days,
      mass: 9.54791938424326609e-4 * solar,
    },
    {
      x: 8.34336671824457987,
      y: 4.12479856412430479,
      z: -4.03523417114321381e-1,
      vx: -2.76742510726862411e-3 * days,
      vy: 4.99852801234917238e-3 * days,
      vz: 2.30417297573763929e-5 * days,
      mass: 2.85885980666130812e-4 * solar,
    },
    {
      x: 1.2894369562139131e1,
      y: -1.51111514016986312e1,
      z: -2.23307578892655734e-1,
      vx: 2.96460137564761618e-3 * days,
      vy: 2.3784717395948095e-3 * days,
      vz: -2.96589568540237556e-5 * days,
      mass: 4.36624404335156298e-5 * solar,
    },
    {
      x: 1.53796971148509165e1,
      y: -2.59193146099879641e1,
      z: 1.79258772950371181e-1,
      vx: 2.68067772490389322e-3 * days,
      vy: 1.62824170038242295e-3 * days,
      vz: -9.5159225451971587e-5 * days,
      mass: 5.15138902046611451e-5 * solar,
    },
  ];
  let px = 0;
  let py = 0;
  let pz = 0;
  for (const b of bodies) {
    px += b.vx * b.mass;
    py += b.vy * b.mass;
    pz += b.vz * b.mass;
  }
  bodies[0].vx = -px / solar;
  bodies[0].vy = -py / solar;
  bodies[0].vz = -pz / solar;
  return bodies;
}

function advance(bodies: Body[], dt: number): void {
  const n = bodies.length;
  for (let i = 0; i < n; i++) {
    const a = bodies[i];
    for (let j = i + 1; j < n; j++) {
      const b = bodies[j];
      const dx = a.x - b.x;
      const dy = a.y - b.y;
      const dz = a.z - b.z;
      const d2 = dx * dx + dy * dy + dz * dz;
      const mag = dt / (d2 * Math.sqrt(d2));
      const mi = a.mass * mag;
      const mj = b.mass * mag;
      a.vx -= dx * mj;
      a.vy -= dy * mj;
      a.vz -= dz * mj;
      b.vx += dx * mi;
      b.vy += dy * mi;
      b.vz += dz * mi;
    }
  }
  for (const b of bodies) {
    b.x += dt * b.vx;
    b.y += dt * b.vy;
    b.z += dt * b.vz;
  }
}

function energy(bodies: Body[]): number {
  let e = 0;
  const n = bodies.length;
  for (let i = 0; i < n; i++) {
    const b = bodies[i];
    e += 0.5 * b.mass * (b.vx * b.vx + b.vy * b.vy + b.vz * b.vz);
    for (let j = i + 1; j < n; j++) {
      const o = bodies[j];
      const dx = b.x - o.x;
      const dy = b.y - o.y;
      const dz = b.z - o.z;
      e -= (b.mass * o.mass) / Math.sqrt(dx * dx + dy * dy + dz * dz);
    }
  }
  return e;
}

const bodies = system();
console.log(Math.floor(energy(bodies) * 1e9));
for (let step = 0; step < 5000000; step++) {
  advance(bodies, 0.01);
}
console.log(Math.floor(energy(bodies) * 1e9));
