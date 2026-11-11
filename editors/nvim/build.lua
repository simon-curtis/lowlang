local root = vim.fn.fnamemodify(debug.getinfo(1, "S").source:sub(2), ":p:h")
local source = vim.fs.joinpath(root, "..", "tree-sitter-low", "src")
local output = vim.fs.joinpath(root, "parser", "low.so")

vim.fn.mkdir(vim.fs.dirname(output), "p")

local result = vim.system({
  "cc", "-shared", "-fPIC", "-O2",
  "-I", source,
  vim.fs.joinpath(source, "parser.c"),
  vim.fs.joinpath(source, "scanner.c"),
  "-o", output,
}):wait()

if result.code ~= 0 then
  error(string.format("building the Low parser failed:\n%s", result.stderr))
end

print(string.format("built %s", output))
