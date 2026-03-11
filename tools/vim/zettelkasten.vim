" Zettelkasten Vim Integration
" Source this file in your .vimrc: source /path/to/zk/tools/vim/zettelkasten.vim

let s:zk_cmd = get(g:, 'zettelkasten_zk_cmd', 'zk')

function! s:detect_root(path) abort
    if exists('g:zettelkasten_root') && !empty(g:zettelkasten_root)
        return fnamemodify(g:zettelkasten_root, ':p')
    endif

    let l:path = empty(a:path) ? getcwd() : a:path
    if filereadable(l:path)
        let l:path = fnamemodify(l:path, ':h')
    endif

    while 1
        if isdirectory(l:path . '/.zk')
            return l:path
        endif
        if isdirectory(l:path . '/.git') || filereadable(l:path . '/.git')
            return l:path
        endif
        if isdirectory(l:path . '/zettels')
            return l:path
        endif

        let l:parent = fnamemodify(l:path, ':h')
        if l:parent ==# l:path
            return getcwd()
        endif
        let l:path = l:parent
    endwhile
endfunction

function! s:zk_command(args) abort
    let l:root = s:detect_root(expand('%:p'))
    return s:zk_cmd . ' --dir ' . shellescape(l:root) . ' ' . a:args
endfunction

function! ZkNew() abort
    let l:title = input('Note Title: ')
    if !empty(l:title)
        execute '!' . s:zk_command('new ' . shellescape(l:title))
    endif
endfunction

function! ZkInsertLink() abort
    let l:link = system(s:zk_command('link'))
    let l:link = substitute(l:link, '\n\+$', '', '')

    if !empty(l:link)
        execute 'normal! a' . l:link
    endif
endfunction

nnoremap <leader>zn :call ZkNew()<CR>
inoremap <C-l> <C-o>:call ZkInsertLink()<CR>
