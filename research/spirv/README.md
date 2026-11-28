# SPIR-V lowering for kernels

Hand-written SPIR-V for the SSA that `low ir` prints for the functions in `kernels.low`. Each module shows one part of the lowering a `compiler/spirv` back end needs.

```
$ python3 gen.py .
$ for k in saxpy scale blur pick both scan; do spirv-as --target-env vulkan1.2 $k.spvasm -o $k.spv && spirv-val --target-env vulkan1.2 $k.spv; done
```

`gen.py` writes the `.spvasm` files. All but `both_naive` pass `spirv-val`. None has run on a device.

| Module | Shows |
|---|---|
| `saxpy` | A slice is a storage buffer of `u32` words. A byte offset becomes a word index, and a load or store of an `f32` is a bitcast. A failed bounds check stores 1 to the flag buffer at binding 0 and returns. |
| `scale` | A slice of 24-byte structs keeps Low's layout: one 8-byte word per field. The module writes the low 4 bytes of a field, where the CPU build writes all 8. |
| `blur` | Two nested loops. The four-deep `&&` chain shares one empty exit block, which the merge phis absorb. |
| `pick` | A pointer that may be one of two buffers is a selector value and an `OpSwitch` at the load. |
| `both` | A shared `else` that does work: each merge carries a `bool` phi saying the `then` arm ran, and the `else` runs once after the outer merge. |
| `both_naive` | The same function with the inner `if` branching straight to the outer `else`, which `spirv-val` rejects. |
| `scan` | `continue` and `break` from nested `if`s branch to the loop's continue block and merge block. |

Scalars and slice lengths are push constants, with the invocation count first. An invocation whose index is not below the count returns at once. `mul` and `add` on `f32` carry `NoContraction`.

The SSA passes a `mut` slice as a pointer to its header and computes offsets in 64 bits. The modules take the buffer and its length directly and compute offsets in 32 bits.
