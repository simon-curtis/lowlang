const counts = new Map<string, number>();
let length = 0;
for (let i = 0; i < 1000000; i++) {
  const word = `w${i % 5000}-${i % 7}`;
  length += word.length;
  counts.set(word, (counts.get(word) ?? 0) + 1);
}
let points = 0;
let matches = 0;
for (const [word, count] of counts) {
  for (const c of word) {
    points += c.codePointAt(0)!;
  }
  if (word === "w42-0") matches += count;
}
console.log(counts.size);
console.log(length);
console.log(points);
console.log(matches);
