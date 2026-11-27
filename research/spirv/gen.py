import sys, os
out = sys.argv[1]

def header(nbuf, pc, nocontract):
    L = ["OpCapability Shader", "OpMemoryModel Logical GLSL450",
         'OpEntryPoint GLCompute %main "main" %gid %pc ' + " ".join(f"%b{i}" for i in range(nbuf + 1)),
         "OpExecutionMode %main LocalSize 64 1 1",
         "OpDecorate %gid BuiltIn GlobalInvocationId",
         "OpDecorate %words ArrayStride 4", "OpDecorate %Buf Block",
         "OpMemberDecorate %Buf 0 Offset 0", "OpDecorate %PC Block"]
    for i in range(nbuf + 1):
        L += [f"OpDecorate %b{i} DescriptorSet 0", f"OpDecorate %b{i} Binding {i}"]
    for i in range(len(pc)):
        L.append(f"OpMemberDecorate %PC {i} Offset {4*i}")
    for n in nocontract:
        L.append(f"OpDecorate %{n} NoContraction")
    L += ["%void = OpTypeVoid", "%fnty = OpTypeFunction %void", "%bool = OpTypeBool",
          "%u32 = OpTypeInt 32 0", "%f32 = OpTypeFloat 32", "%v3u = OpTypeVector %u32 3",
          "%p_in_v3u = OpTypePointer Input %v3u", "%gid = OpVariable %p_in_v3u Input",
          "%p_in_u32 = OpTypePointer Input %u32", "%words = OpTypeRuntimeArray %u32",
          "%Buf = OpTypeStruct %words", "%p_buf = OpTypePointer StorageBuffer %Buf",
          "%p_sb_u32 = OpTypePointer StorageBuffer %u32"]
    for i in range(nbuf + 1):
        L.append(f"%b{i} = OpVariable %p_buf StorageBuffer")
    L.append("%PC = OpTypeStruct " + " ".join("%" + t for _, t in pc))
    L += ["%p_pc = OpTypePointer PushConstant %PC", "%pc = OpVariable %p_pc PushConstant",
          "%p_pc_u32 = OpTypePointer PushConstant %u32", "%p_pc_f32 = OpTypePointer PushConstant %f32"]
    for k in (0, 1, 2, 4, 24):
        L.append(f"%c{k} = OpConstant %u32 {k}")
    L += ["%f0 = OpConstant %f32 0", "%f1 = OpConstant %f32 1",
          "%main = OpFunction %void None %fnty", "%entry = OpLabel",
          "%gp = OpAccessChain %p_in_u32 %gid %c0", "%id = OpLoad %u32 %gp"]
    for i, (n, t) in enumerate(pc):
        L.append(f"%ix_{n} = OpConstant %u32 {i}")
    L2 = []
    for i, (n, t) in enumerate(pc):
        L2 += [f"%pp_{n} = OpAccessChain %p_pc_{t} %pc %ix_{n}", f"%{n} = OpLoad %{t} %pp_{n}"]
    # constants must precede the function
    consts = [l for l in L if l.startswith("%ix_")]
    L = [l for l in L if not l.startswith("%ix_")]
    at = L.index("%main = OpFunction %void None %fnty")
    L = L[:at] + consts + L[at:] + L2
    L += ["%inrange = OpULessThan %bool %id %count", "OpSelectionMerge %start None",
          "OpBranchConditional %inrange %start %skip", "%skip = OpLabel", "OpReturn", "%start = OpLabel"]
    return L

uid = [0]
def check(idx, limit):
    uid[0] += 1; k = uid[0]
    return [f"%ok{k} = OpULessThan %bool %{idx} %{limit}", f"OpSelectionMerge %cm{k} None",
            f"OpBranchConditional %ok{k} %cm{k} %cf{k}", f"%cf{k} = OpLabel",
            f"%fp{k} = OpAccessChain %p_sb_u32 %b0 %c0 %c0", f"OpStore %fp{k} %c1", "OpReturn",
            f"%cm{k} = OpLabel"], f"cm{k}"

def addr(name, buf, off, extra=None):
    L = [f"%{name}_w = OpShiftRightLogical %u32 %{off} %c2"]
    w = f"{name}_w"
    if extra:
        L.append(f"%{name}_w2 = OpIAdd %u32 %{w} %{extra}"); w = f"{name}_w2"
    L.append(f"%{name}_p = OpAccessChain %p_sb_u32 %{buf} %c0 %{w}")
    return L
def loadf(name, buf, off, extra=None):
    return addr(name, buf, off, extra) + [f"%{name}_u = OpLoad %u32 %{name}_p", f"%{name} = OpBitcast %f32 %{name}_u"]
def loadu(name, buf, off, extra=None):
    return addr(name, buf, off, extra) + [f"%{name} = OpLoad %u32 %{name}_p"]
def storef(name, buf, off, val, extra=None):
    return addr(name, buf, off, extra) + [f"%{name}_u = OpBitcast %u32 %{val}", f"OpStore %{name}_p %{name}_u"]
def storeu(name, buf, off, val, extra=None):
    return addr(name, buf, off, extra) + [f"OpStore %{name}_p %{val}"]

END = ["OpReturn", "OpFunctionEnd"]
def write(name, lines):
    open(os.path.join(out, name + ".spvasm"), "w").write("\n".join(lines) + "\n")

# saxpy: y[id] = a * x[id] + y[id]
L = header(2, [("count","u32"),("a","f32"),("xlen","u32"),("ylen","u32")], ["m","s"])
c, _ = check("id", "xlen"); L += c
L += ["%off = OpShiftLeftLogical %u32 %id %c2"] + loadf("xv", "b1", "off")
L += ["%m = OpFMul %f32 %a %xv"]
c, _ = check("id", "ylen"); L += c
L += loadf("yv", "b2", "off") + ["%s = OpFAdd %f32 %m %yv"] + storef("st", "b2", "off", "s") + END
write("saxpy", L)

# scale: 24-byte structs, x at byte 0, n at byte 16
L = header(1, [("count","u32"),("k","f32"),("pslen","u32")], ["m"])
c, _ = check("id", "pslen"); L += c
L += ["%off = OpIMul %u32 %id %c24"] + loadf("x", "b1", "off") + ["%m = OpFMul %f32 %x %k"]
L += storef("sx", "b1", "off", "m")
c, _ = check("id", "pslen"); L += c
L += loadu("n", "b1", "off", "c4") + ["%n1 = OpIAdd %u32 %n %c1"] + storeu("sn", "b1", "off", "n1", "c4") + END
write("scale", L)

# pick: phi of two buffers becomes (selector, length) phis and a switch at the load
L = header(3, [("count","u32"),("flag","u32"),("alen","u32"),("blen","u32"),("outlen","u32")], [])
L += ["%z = OpIEqual %bool %flag %c0", "OpSelectionMerge %j None", "OpBranchConditional %z %ta %tb",
      "%ta = OpLabel", "OpBranch %j", "%tb = OpLabel", "OpBranch %j", "%j = OpLabel",
      "%sel = OpPhi %u32 %c1 %ta %c2 %tb", "%len = OpPhi %u32 %alen %ta %blen %tb"]
c, _ = check("id", "len"); L += c
L += ["%off = OpShiftLeftLogical %u32 %id %c2", "OpSelectionMerge %lm None", "OpSwitch %sel %lb 1 %la",
      "%la = OpLabel"] + loadf("va", "b1", "off") + ["OpBranch %lm", "%lb = OpLabel"] + loadf("vb", "b2", "off")
L += ["OpBranch %lm", "%lm = OpLabel", "%v = OpPhi %f32 %va %la %vb %lb"]
c, _ = check("id", "outlen"); L += c
L += storef("so", "b3", "off", "v") + END
write("pick", L)

# blur: two loops, a four-deep && chain whose shared exit is threaded through merge phis
L = header(2, [("count","u32"),("w","u32"),("h","u32"),("r","u32"),("srclen","u32"),("dstlen","u32")], ["sum2","n2","avg"])
L += ["%x = OpUMod %u32 %id %w", "%y = OpUDiv %u32 %id %w", "%r2 = OpShiftLeftLogical %u32 %r %c1", "OpBranch %L1",
      "%L1 = OpLabel",
      "%n = OpPhi %f32 %f0 %start %n5 %L1c", "%sum = OpPhi %f32 %f0 %start %sum5 %L1c", "%dy = OpPhi %u32 %c0 %start %dy1 %L1c",
      "OpLoopMerge %L1m %L1c None", "OpBranch %L1t", "%L1t = OpLabel",
      "%t1 = OpULessThanEqual %bool %dy %r2", "OpBranchConditional %t1 %B2 %L1m",
      "%B2 = OpLabel", "%sy = OpIAdd %u32 %y %dy", "%syr = OpISub %u32 %sy %r", "OpBranch %L5",
      "%L5 = OpLabel",
      "%n5 = OpPhi %f32 %n %B2 %n6 %L5c", "%sum5 = OpPhi %f32 %sum %B2 %sum6 %L5c", "%dx = OpPhi %u32 %c0 %B2 %dx1 %L5c",
      "OpLoopMerge %L5m %L5c None", "OpBranch %L5t", "%L5t = OpLabel",
      "%t5 = OpULessThanEqual %bool %dx %r2", "OpBranchConditional %t5 %B6 %L5m",
      "%B6 = OpLabel", "%sx = OpIAdd %u32 %x %dx", "%q1 = OpUGreaterThanEqual %bool %sx %r",
      "OpSelectionMerge %M6 None", "OpBranchConditional %q1 %B16 %M6",
      "%B16 = OpLabel", "%q2 = OpUGreaterThanEqual %bool %sy %r",
      "OpSelectionMerge %M16 None", "OpBranchConditional %q2 %B14 %M16",
      "%B14 = OpLabel", "%sxr = OpISub %u32 %sx %r", "%q3 = OpULessThan %bool %sxr %w",
      "OpSelectionMerge %M14 None", "OpBranchConditional %q3 %B12 %M14",
      "%B12 = OpLabel", "%q4 = OpULessThan %bool %syr %h",
      "OpSelectionMerge %M12 None", "OpBranchConditional %q4 %B9 %M12",
      "%B9 = OpLabel", "%row = OpIMul %u32 %syr %w", "%ti = OpIAdd %u32 %row %sxr"]
c, last = check("ti", "srclen"); L += c
L += ["%toff = OpShiftLeftLogical %u32 %ti %c2"] + loadf("tv", "b1", "toff")
L += ["%sum2 = OpFAdd %f32 %sum5 %tv", "%n2 = OpFAdd %f32 %n5 %f1", "OpBranch %M12",
      "%M12 = OpLabel", f"%sA = OpPhi %f32 %sum2 %{last} %sum5 %B12", f"%nA = OpPhi %f32 %n2 %{last} %n5 %B12", "OpBranch %M14",
      "%M14 = OpLabel", "%sB = OpPhi %f32 %sA %M12 %sum5 %B14", "%nB = OpPhi %f32 %nA %M12 %n5 %B14", "OpBranch %M16",
      "%M16 = OpLabel", "%sC = OpPhi %f32 %sB %M14 %sum5 %B16", "%nC = OpPhi %f32 %nB %M14 %n5 %B16", "OpBranch %M6",
      "%M6 = OpLabel", "%sum6 = OpPhi %f32 %sC %M16 %sum5 %B6", "%n6 = OpPhi %f32 %nC %M16 %n5 %B6", "OpBranch %L5c",
      "%L5c = OpLabel", "%dx1 = OpIAdd %u32 %dx %c1", "OpBranch %L5",
      "%L5m = OpLabel", "OpBranch %L1c",
      "%L1c = OpLabel", "%dy1 = OpIAdd %u32 %dy %c1", "OpBranch %L1",
      "%L1m = OpLabel", "%avg = OpFDiv %f32 %sum %n"]
c, _ = check("id", "dstlen"); L += c
L += ["%doff = OpShiftLeftLogical %u32 %id %c2"] + storef("sd", "b2", "doff", "avg") + END
write("blur", L)

def rw(tag, op):
    L = []
    c, _ = check("id", "srclen"); L += c
    L += [f"%off{tag} = OpShiftLeftLogical %u32 %id %c2"] + loadf(f"v{tag}", "b1", f"off{tag}")
    L += [f"%r{tag} = {op} %f32 %v{tag} %f1"]
    c, last = check("id", "dstlen"); L += c
    L += storef(f"s{tag}", "b2", f"off{tag}", f"r{tag}")
    return L, last
PCB = [("count","u32"),("a","u32"),("b","u32"),("srclen","u32"),("dstlen","u32")]
T = ["%true = OpConstantTrue %bool", "%false = OpConstantFalse %bool"]
def with_consts(L):
    at = L.index("%main = OpFunction %void None %fnty")
    return L[:at] + T + L[at:]

# both, naive: the inner header branches straight to the outer else. Expected to be rejected.
L = with_consts(header(2, PCB, []))
L += ["%q1 = OpULessThan %bool %id %a", "OpSelectionMerge %B3 None", "OpBranchConditional %q1 %B4 %B2",
      "%B4 = OpLabel", "%q2 = OpULessThan %bool %id %b", "OpSelectionMerge %M4 None", "OpBranchConditional %q2 %B1 %B2",
      "%B1 = OpLabel"]
w, _ = rw("t", "OpFMul"); L += w + ["OpBranch %M4", "%M4 = OpLabel", "OpBranch %B3", "%B2 = OpLabel"]
w, _ = rw("e", "OpFAdd"); L += w + ["OpBranch %B3", "%B3 = OpLabel"] + END
write("both_naive", L)

# both, predicate phi: each merge carries "the then arm ran", and the shared else runs once after
L = with_consts(header(2, PCB, []))
L += ["%q1 = OpULessThan %bool %id %a", "OpSelectionMerge %M0 None", "OpBranchConditional %q1 %B4 %M0",
      "%B4 = OpLabel", "%q2 = OpULessThan %bool %id %b", "OpSelectionMerge %M4 None", "OpBranchConditional %q2 %B1 %M4",
      "%B1 = OpLabel"]
w, last = rw("t", "OpFMul")
L += w + ["OpBranch %M4", "%M4 = OpLabel", f"%d1 = OpPhi %bool %true %{last} %false %B4", "OpBranch %M0",
          "%M0 = OpLabel", "%d = OpPhi %bool %d1 %M4 %false %start",
          "OpSelectionMerge %B3 None", "OpBranchConditional %d %B3 %B2", "%B2 = OpLabel"]
w, _ = rw("e", "OpFAdd"); L += w + ["OpBranch %B3", "%B3 = OpLabel"] + END
write("both", L)

# scan: continue and break from inside nested ifs go straight to the continue block and the loop merge
L = header(2, [("count","u32"),("n","u32"),("srclen","u32"),("dstlen","u32")], ["sum2"])
L += ["%f100 = OpConstant %f32 100"]
at = L.index("%main = OpFunction %void None %fnty"); L = L[:at] + [L[-1]] + L[at:-1]
L += ["OpBranch %L1", "%L1 = OpLabel", "%sum = OpPhi %f32 %f0 %start %sum5 %L1c", "%i = OpPhi %u32 %c0 %start %i1 %L1c",
      "OpLoopMerge %L1m %L1c None", "OpBranch %L1t", "%L1t = OpLabel",
      "%t1 = OpULessThan %bool %i %n", "OpBranchConditional %t1 %L5 %L1m",
      "%L5 = OpLabel", "%sum5 = OpPhi %f32 %sum %L1t %sum7 %L5c", "%j = OpPhi %u32 %c0 %L1t %j1 %L5c",
      "OpLoopMerge %L5m %L5c None", "OpBranch %L5t", "%L5t = OpLabel",
      "%t5 = OpULessThan %bool %j %n", "OpBranchConditional %t5 %B6 %L5m",
      "%B6 = OpLabel", "%j1 = OpIAdd %u32 %j %c1"]
c, last = check("j1", "srclen"); L += c
L += ["%off = OpShiftLeftLogical %u32 %j1 %c2"] + loadf("v", "b1", "off")
L += ["%neg = OpFOrdLessThan %bool %v %f0", "OpSelectionMerge %B10 None", "OpBranchConditional %neg %L5c %B10",
      "%B10 = OpLabel", "%big = OpFOrdGreaterThan %bool %v %f100", "OpSelectionMerge %B14 None", "OpBranchConditional %big %L5m %B14",
      "%B14 = OpLabel", "%sum2 = OpFAdd %f32 %sum5 %v", "OpBranch %L5c",
      "%L5c = OpLabel", f"%sum7 = OpPhi %f32 %sum5 %{last} %sum2 %B14", "OpBranch %L5",
      "%L5m = OpLabel", "OpBranch %L1c",
      "%L1c = OpLabel", "%i1 = OpIAdd %u32 %i %c1", "OpBranch %L1",
      "%L1m = OpLabel"]
c, _ = check("id", "dstlen"); L += c
L += ["%doff = OpShiftLeftLogical %u32 %id %c2"] + storef("sd", "b2", "doff", "sum") + END
write("scan", L)
