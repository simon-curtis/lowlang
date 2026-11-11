function applyAll(xs: number[], f: (x: number) => number): number {
  let total = 0;
  for (const x of xs) total += f(x);
  let lowBits = 0;
  for (const x of xs) lowBits += f(x) & 1;
  return total + lowBits;
}

function countIf(xs: number[], keep: (x: number) => boolean): number {
  let n = 0;
  for (const x of xs) {
    if (keep(x)) n++;
  }
  let rest = 0;
  for (const x of xs) {
    if (!keep(x)) rest += 2;
  }
  return n + rest;
}

const xs: number[] = [];
for (let i = 0; i < 2000; i++) xs.push(i);
let total = 0;
let hits = 0;
for (let round = 0; round < 5000; round++) {
  total += applyAll(xs, (x) => x * 3 + 1);
  total += applyAll(xs, (x) => (x ^ round) & 1023);
  total += applyAll(xs, (x) => x >> 1);
  hits += countIf(xs, (x) => x % 7 === 0);
  hits += countIf(xs, (x) => x > round % 2000);
}
console.log(total);
console.log(hits);
