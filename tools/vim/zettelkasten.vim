" Zettelkasten Vim Integration
" Source this file in your .vimrc: source /path/to/zettelkasten/tools/vim/zettelkasten.vim

let s:zk_path = getcwd() . '/bin/zk'

function! ZkNew()
    let title = input('Note Title: ')
    if title != ''
        " Run in a terminal or shell
        execute '!' . s:zk_path . ' new ' . shellescape(title)
    endif
endfunction

function! ZkInsertLink()
    " Call the python script which uses fzf
    " We use system() to capture the output
    let link = system(s:zk_path . ' link')
    
    " If a link was selected (output is not empty), insert it
    if link != ''
        " Remove potential newline characters if the script added them (though we tried to suppress)
        let link = substitute(link, '\n', '', 'g')
        execute "normal! a" . link
    endif
endfunction

" Mappings
" Create new note
nnoremap <leader>zn :call ZkNew()<CR>
" Insert link (Ctrl+L in insert mode)
inoremap <C-l> <C-o>:call ZkInsertLink()<CR>

