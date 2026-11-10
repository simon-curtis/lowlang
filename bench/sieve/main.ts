function sieve(limit: number): number {
  const composite = new Uint8Array(limit);
  let count = 0;
  for (let i = 2; i < limit; i++) {
    if (composite[i] === 0) {
      count++;
      for (let j = i * i; j < limit; j += i) {
        composite[j] = 1;
      }
    }
  }
  return count;
}

let total = 0;
for (let round = 0; round < 5; round++) {
  total += sieve(10000000);
}
console.log(total);
