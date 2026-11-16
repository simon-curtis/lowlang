# Layout oracle trees

`oracle_trees.txt` holds trees of flex nodes and `oracle_golden_<scale>.txt` the border boxes taffy 0.15 gives them when each authored length is rounded to device pixels as gpui-fast 0.1.12 rounds it (`taffy.rs`) and each box is snapped as `layout_bounds` snaps it. The scales are 100, 125, 150 and 175 percent, in a 640 by 480 logical window. `std/ui/layout/oracle_test.low` lays out each tree with `std/ui/layout` and requires every rectangle to be bit-equal.

A tree is a `TREE <name>` line followed by one line per node in depth-first order: the depth, then `key=value` words. The node at depth 0 is the root; an `Auto` root size is the window.

| key | value |
| --- | --- |
| `dir` | `row` or `col` |
| `wrap` | `none` or `wrap` |
| `justify`, `items`, `self`, `content` | `start`, `end`, `center`, `stretch`, `between`, `around`, `evenly` |
| `grow`, `shrink` | a number |
| `basis`, `w`, `h`, `minw`, `minh`, `maxw`, `maxh` | `auto`, a length or a percentage |
| `m`, `inset` | one or four (top, right, bottom, left) of `auto`, a length or a percentage |
| `p`, `b` | one or four lengths |
| `gap` | one length, or column gap and row gap |
| `ox`, `oy` | `visible`, `hidden` or `scroll` |
| `pos` | `abs` |
| `display` | `none` |
| `leaf` | `<word widths separated by ;>\|<line height>`: a leaf whose words wrap at a known or definite width, never when asked for its narrowest or widest size, with a space of 4 |

A golden file has a `RESULT <name> <scale>` line per tree and then `<index> <x> <y> <width> <height>` per node in the tree's order, in logical pixels.
