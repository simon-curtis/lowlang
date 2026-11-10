vim.bo.commentstring = "// %s"
vim.bo.expandtab = true
vim.bo.shiftwidth = 4
vim.bo.tabstop = 4

local started = pcall(vim.treesitter.start)
if started then
  vim.wo[0][0].foldmethod = "expr"
  vim.wo[0][0].foldexpr = "v:lua.vim.treesitter.foldexpr()"
  vim.wo[0][0].foldlevel = 99
end

local nvim_treesitter = pcall(require, "nvim-treesitter")
if started and nvim_treesitter then
  vim.bo.indentexpr = "v:lua.require'nvim-treesitter'.indentexpr()"
else
  vim.bo.cindent = true
end

vim.b.undo_ftplugin = "setlocal commentstring< expandtab< shiftwidth< tabstop< indentexpr< cindent<"
