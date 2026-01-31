-- Neovim LSP Configuration using nvim-lspconfig
-- Add this to your init.lua

local lspconfig = require('lspconfig')
local configs = require('lspconfig.configs')

if not configs.zettelkasten then
  configs.zettelkasten = {
    default_config = {
      cmd = { 'zk', 'lsp' }, -- Ensure 'zk' is in your PATH
      filetypes = { 'markdown' },
      root_dir = lspconfig.util.root_pattern('.zk', '.git'),
      settings = {},
    },
  }
end

lspconfig.zettelkasten.setup{}
