function quicksort<T>(xs: T[], lo: number, hi: number, less: (a: T, b: T) => boolean): void {
  if (hi - lo < 2) return;
  const pivot = xs[lo + ((hi - lo) >> 1)];
  let i = lo;
  let j = hi - 1;
  while (i <= j) {
    while (less(xs[i], pivot)) i++;
    while (less(pivot, xs[j])) j--;
    if (i <= j) {
      const t = xs[i];
      xs[i] = xs[j];
      xs[j] = t;
      i++;
      j--;
    }
  }
  quicksort(xs, lo, j + 1, less);
  quicksort(xs, i, hi, less);
}

const n = 3000000;
const xs: number[] = [];
let seed = 42;
for (let i = 0; i < n; i++) {
  seed = (Math.imul(seed, 1103515245) + 12345) & 0x7fffffff;
  xs.push(seed);
}
quicksort(xs, 0, xs.length, (a, b) => a < b);
let unsorted = 0;
let checksum = 0;
for (let i = 0; i < n; i++) {
  if (i > 0 && xs[i - 1] > xs[i]) unsorted++;
  if (i % 1000 === 0) checksum += xs[i];
}
console.log(unsorted);
console.log(checksum);
