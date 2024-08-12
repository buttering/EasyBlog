---
title: vim 粘贴后开头字符丢失
date: 2024-08-12 10:45:20
toc: true
mathjax: true
tags: 
- vim
- linux
---

## 描述
https://stackoverflow.com/questions/19544680/losing-text-while-pasting-into-vi

向 vim 中粘贴文本（尤其是长文本）时，有时会出现开头部分字符丢失的问题。有时候，还会发现最前面的部分字符还会移动到文本的最后。
经过验证，排除了终端、用户、远程机器的问题。

## 解决办法
在进入了 paste 模式后
```shell
:set paste
```
还要按`i`进入插入模式，这样才能正常进行复制粘贴。

附vim配置
```shell
" 基础 Vim 配置 from GPT-4

" 设置与vi不兼容，仅使用vim
set nocompatible

" 设置历史记录条目数
set history=1000

" 启用行号显示
set number

" 高亮当前行
set cursorline

" 启用语法高亮
syntax on
syntax enable

" 设置文件编码和换行格式
set encoding=utf-8
set fileencodings=utf-8,gbk,gb2312,cp936
set fileformats=unix,dos

" 设置tab键的宽度
set tabstop=4
set shiftwidth=4
set expandtab

" 在行和段落上更聪明的移动
set whichwrap+=<,>,h,l,[,]

" 启动自动换行
set wrap

" 启用鼠标支持
" set mouse=a

" 开启实时搜索功能
set incsearch

" 搜索时大小写不敏感，除非包含大写字母
set ignorecase
set smartcase

" 使回退可以跨越插入点
set backspace=indent,eol,start

" 启用自动缩进
set autoindent
set smartindent

" 在窗口底部显示标尺，显示光标的当前位置
set ruler

" 显示不可见字符
set list
set listchars=tab:»·,trail:·

" 设置折叠级别，并对c和python开启折叠功能
set foldlevel=100
filetype on
autocmd FileType c,py setl fdm=syntax | setl fen

" 配置备份选项
set backup
set backupdir=~/.vim/backups
set undodir=~/.vim/undo
set undofile

" 为多种文件类型启用文件类型插件
filetype plugin on
filetype indent on

" 启用颜色主题
colorscheme elflord
" highlight Search ctermfg=grey ctermbg=darkblue 
highlight Comment ctermfg=grey ctermbg=none

" 开启搜索匹配项高亮显示
" highlight Search ctermfg=grey ctermbg=darkblue
set hlsearch

" 自定义快捷键
" 映射 ; 作为命令行前缀，避免按 Shift 键
nnoremap ; :

" 映射 Ctrl+S 为保存
nnoremap <C-S> :w<CR>
inoremap <C-S> <Esc>:w<CR>a

" 映射 Ctrl+Z 为撤销
nnoremap <C-Z> u
inoremap <C-Z> <Esc>u

" 映射 Ctrl+Y 为重做
nnoremap <C-Y> <C-R>
inoremap <C-Y> <Esc><C-R>

" 用于粘贴板的更好的复制和粘贴
" 在普通模式下用 Ctrl+C 复制到系统粘贴板
vnoremap <C-C> "+y
" 在插入模式下用 Ctrl+V 粘贴系统粘贴板内容
inoremap <C-V> <C-R>

" 如果需要插件管理器，例如 Vim-Plug，可以添加以下配置
" curl -fLo ~/.vim/autoload/plug.vim --create-dirs \
"    https://raw.githubusercontent.com/junegunn/vim-plug/master/plug.vim

 call plug#begin('~/.vim/plugged')
 Plug 'tpope/vim-fugitive'
 Plug 'airblade/vim-gitgutter'
 call plug#end()

" 设置文件创建或修改时，自动生成文件头部描述信息（owner、创建时间、修改者等信息）
autocmd BufNewFile *.sh,*.pl,*.py exec ":call SetTitle()"
"autocmd BufWrite   *.sh,*.pl,*.py exec ":call ModifyTitle()"
autocmd BufWrite *.sh,*pl,*py if getline(6) != "# Modify Author: ".expand("$USER@alibaba-inc.com") || split(getline(7))[3] != strftime("%F") | call ModifyTitle() | endif

autocmd BufNewFile,BufRead *.py exec ":call SetTable()"
func SetTable()
    set expandtab
    set tabstop=4
    set shiftwidth=4
endfunc

func SetTitle()
    if &filetype == 'sh'
        call setline(1, "\#!/bin/sh")
        call append(line("."), "\#****************************************************************#")
        call append(line(".")+1, "\# ScriptName: ".expand("%") )
        call append(line(".")+2, "\# Author: ".expand("$USER@alibaba-inc.com") )
        call append(line(".")+3, "\# Create Date: ".strftime("%F %R"))
        call append(line(".")+4, "\# Modify Author: ".expand("$USER@alibaba-inc.com") )
        call append(line(".")+5, "\# Modify Date: ".strftime("%F %R"))
        call append(line(".")+6, "\# Description: " )
        call append(line(".")+7, "\#***************************************************************#")
        call append(line(".")+8, "")
        :8
    elseif &filetype == 'perl'
        call setline(1, "\#!/usr/bin/perl")
        call append(line("."), "\#****************************************************************#")
        call append(line(".")+1, "\# ScriptName: ".expand("%") )
        call append(line(".")+2, "\# Author: ".expand("$USER@alibaba-inc.com") )
        call append(line(".")+3, "\# Create Date: ".strftime("%F %R"))
        call append(line(".")+4, "\# Modify Author: ".expand("$USER@alibaba-inc.com") )
        call append(line(".")+5, "\# Modify Date: ".strftime("%F %R"))
        call append(line(".")+6, "\# Description: ")
        call append(line(".")+7, "\#***************************************************************#")
        call append(line(".")+8, "")
        :8
    elseif &filetype == 'python'
        call setline(1, "\#!/usr/bin/python")
        call append(line("."), "\#****************************************************************#")
        call append(line(".")+1, "\# ScriptName: ".expand("%") )
        call append(line(".")+2, "\# Author: ".expand("$USER@alibaba-inc.com") )
        call append(line(".")+3, "\# Create Date: ".strftime("%F %R"))
        call append(line(".")+4, "\# Modify Author: ".expand("$USER@alibaba-inc.com") )
        call append(line(".")+5, "\# Modify Date: ".strftime("%F %R"))
        call append(line(".")+6, "\# Description: ")
        call append(line(".")+7, "\#***************************************************************#")
        call append(line(".")+8, "")
        :8
    endif
endfunc

func ModifyTitle()
    if getline(6) =~ "# Modify Author:.*"
        call setline(6, "\# Modify Author: ".expand("$USER@alibaba-inc.com") )
        call setline(7, "\# Modify Date: ".strftime("%F %R"))
    endif
endfunc

" 设置paste模式,不自动缩进
set paste

" 注意：上面的插件管理器部分已被注释，如需使用请取消注释并安装插件
```
