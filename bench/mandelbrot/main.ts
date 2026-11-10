function escapes(cr: number, ci: number, limit: number): boolean {
  let zr = 0;
  let zi = 0;
  for (let i = 0; i < limit; i++) {
    const tr = zr * zr - zi * zi + cr;
    zi = 2 * zr * zi + ci;
    zr = tr;
    if (zr * zr + zi * zi > 4) return true;
  }
  return false;
}

const size = 2000;
const scale = 2 / size;
let inside = 0;
for (let y = 0; y < size; y++) {
  const ci = y * scale - 1;
  for (let x = 0; x < size; x++) {
    const cr = x * scale - 1.5;
    if (!escapes(cr, ci, 50)) inside++;
  }
}
console.log(inside);
