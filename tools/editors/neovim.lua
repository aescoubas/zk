-- Neovim LSP Configuration using nvim-lspconfig
-- Add this to your init.lua

local lspconfig = require('lspconfig')
local configs = require('lspconfig.configs')
local util = lspconfig.util

if not configs.zettelkasten then
  configs.zettelkasten = {
    default_config = {
      cmd = { 'zk', 'lsp' },
      filetypes = { 'markdown' },
      root_dir = function(fname)
        return util.root_pattern('.zk', '.git', 'zettels')(fname) or util.path.dirname(fname)
      end,
      on_new_config = function(new_config, new_root_dir)
        new_config.cmd = { 'zk', 'lsp', '--dir', new_root_dir }
      end,
      settings = {},
    },
  }
end

lspconfig.zettelkasten.setup{}
