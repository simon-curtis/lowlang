# Advent of Code 2025 in Low

Solutions to the twelve puzzles of [Advent of Code 2025](https://adventofcode.com/2025), one program per day.

Each program embeds its puzzle input at compile time from `input/dayNN.txt` and prints `part 1: <answer>` and `part 2: <answer>`. Inputs are generated per account and are not redistributed, so `input/` is ignored by git: save your own input from `https://adventofcode.com/2025/day/N/input` as `input/dayNN.txt` (day 1 is `input/day01.txt`), then run

```
low run docs/examples/aoc/2025/day01.low
```

| Day | Program | Puzzle in brief |
|---|---|---|
| [1](https://adventofcode.com/2025/day/1) | [day01.low](day01.low) | Turn a 100-position dial and count how often it rests on, then passes through, zero. |
| [2](https://adventofcode.com/2025/day/2) | [day02.low](day02.low) | Sum the numbers in a set of ranges whose digits are one sequence repeated twice, then repeated any number of times. |
| [3](https://adventofcode.com/2025/day/3) | [day03.low](day03.low) | Pick 2, then 12, digits from each line in order to form the largest number. |
| [4](https://adventofcode.com/2025/day/4) | [day04.low](day04.low) | Count grid cells with fewer than four occupied neighbours, then peel them away until none remain. |
| [5](https://adventofcode.com/2025/day/5) | [day05.low](day05.low) | Count listed IDs inside a set of ranges, then the size of the ranges' union. |
| [6](https://adventofcode.com/2025/day/6) | [day06.low](day06.low) | Evaluate a worksheet of sums and products read by rows, then by columns. |
| [7](https://adventofcode.com/2025/day/7) | [day07.low](day07.low) | Count the splitters a falling beam hits, then the number of paths through them. |
| [8](https://adventofcode.com/2025/day/8) | [day08.low](day08.low) | Join the closest pairs of 3D points into circuits with union-find. |
| [9](https://adventofcode.com/2025/day/9) | [day09.low](day09.low) | Find the largest rectangle with corners on given tiles, then the largest inside their rectilinear loop. |
| [10](https://adventofcode.com/2025/day/10) | [day10.low](day10.low) | Fewest button presses to set lights (XOR), then counters (an integer linear program). |
| [11](https://adventofcode.com/2025/day/11) | [day11.low](day11.low) | Count paths through a directed acyclic graph, then those through two required nodes. |
| [12](https://adventofcode.com/2025/day/12) | [day12.low](day12.low) | Count regions that can hold their set of polyomino presents. Day 12 has one part. |
