class Tree {
  constructor(
    readonly left: Tree | null,
    readonly right: Tree | null,
  ) {}

  check(): number {
    let total = 1;
    if (this.left !== null) total += this.left.check();
    if (this.right !== null) total += this.right.check();
    return total;
  }
}

function make(depth: number): Tree {
  return depth === 0 ? new Tree(null, null) : new Tree(make(depth - 1), make(depth - 1));
}

const maxDepth = 18;
const stretch = maxDepth + 1;
console.log(`stretch tree of depth ${stretch}\t check: ${make(stretch).check()}`);
const longLived = make(maxDepth);
for (let depth = 4; depth <= maxDepth; depth += 2) {
  const iterations = 1 << (maxDepth - depth + 4);
  let sum = 0;
  for (let i = 0; i < iterations; i++) {
    sum += make(depth).check();
  }
  console.log(`${iterations}\t trees of depth ${depth}\t check: ${sum}`);
}
console.log(`long lived tree of depth ${maxDepth}\t check: ${longLived.check()}`);
