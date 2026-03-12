# bash completion for kall

_kall() {
  local cur prev
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"

  case "$prev" in
    kall)
      COMPREPLY=($(compgen -W "init config list alias aliases --help --version" -- "$cur"))
      return
      ;;
    alias)
      # complete with project names from .kall
      local config
      config=$(_kall_find_config)
      if [ -n "$config" ]; then
        local projects
        projects=$(grep '^\[' "$config" | tr -d '[]')
        COMPREPLY=($(compgen -W "$projects" -- "$cur"))
      fi
      return
      ;;
  esac
}

_kall_find_config() {
  local dir="$PWD"
  while [ "$dir" != "/" ]; do
    [ -f "$dir/.kall" ] && echo "$dir/.kall" && return
    dir="$(dirname "$dir")"
  done
}

complete -F _kall kall
