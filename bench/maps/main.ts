const n = 1000000;
const m = new Map<number, number>();
for (let i = 0; i < n; i++) {
  m.set(i * 7, i);
}
let hits = 0;
let sum = 0;
for (let k = 0; k < n * 2; k++) {
  const v = m.get(k) ?? -1;
  if (v >= 0) {
    hits++;
    sum += v;
  }
}
for (let i = 0; i < n; i++) {
  if (i % 2 === 0) m.delete(i * 7);
}
for (let i = 0; i < n / 2; i++) {
  m.set(i * 7 + 1, i);
}
let total = 0;
for (const [k, v] of m) {
  total += k + v;
}
console.log(hits);
console.log(sum);
console.log(m.size);
console.log(total);
