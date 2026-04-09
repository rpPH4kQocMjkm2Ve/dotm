# completions/dotm.bash
# bash completion for dotm

_dotm() {
    local cur prev words cword
    _init_completion || return

    if [[ $cword -eq 1 ]]; then
        COMPREPLY=( $(compgen -W "init apply diff status reset -h --help -V --version" -- "$cur") )
        return
    fi

    case "${words[1]}" in
        apply)
            COMPREPLY=( $(compgen -W "-n --dry-run -h --help" -- "$cur") )
            ;;
        status)
            COMPREPLY=( $(compgen -W "-v --verbose -q --quiet -h --help" -- "$cur") )
            ;;
        diff)
            COMPREPLY=( $(compgen -W "-h --help" -- "$cur") )
            ;;
        init)
            COMPREPLY=( $(compgen -W "-h --help" -- "$cur") )
            ;;
        reset)
            COMPREPLY=( $(compgen -W "--all -h --help" -- "$cur") )
            ;;
        version)
            COMPREPLY=()
            ;;
    esac
}

complete -F _dotm dotm
