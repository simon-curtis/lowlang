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

vim.api.nvim_buf_create_user_command(0, "LowToggleMain", function()
  local client = vim.lsp.get_clients({ bufnr = 0, name = "low" })[1]
  if not client then
    vim.notify("low lsp is not attached", vim.log.levels.WARN)
    return
  end
  local command = { title = "Toggle fn main", command = "low.toggleMain", arguments = { vim.uri_from_bufnr(0) } }
  client:exec_cmd(command, { bufnr = 0 })
end, { desc = "Move the top-level statements into fn main, or fn main to the top level" })

vim.b.undo_ftplugin = "setlocal commentstring< expandtab< shiftwidth< tabstop< indentexpr< cindent< | delcommand -buffer LowToggleMain"
