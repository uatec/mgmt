#compdef mgmt
# zsh completion for mgmt. Load it with:  source <(mgmt completion zsh)

_mgmt() {
  local line value desc
  local -a lines values displays
  local files=0
  # the words after "mgmt", up to and including the one under the cursor
  lines=("${(@f)$(command mgmt __complete "${(@)words[2,CURRENT]}" 2>/dev/null)}")
  for line in "${lines[@]}"; do
    [[ -z $line ]] && continue
    if [[ $line == ':files' ]]; then
      files=1
      continue
    fi
    value=${line%%$'\t'*}
    desc=''
    [[ $line == *$'\t'* ]] && desc=${line#*$'\t'}
    values+=("$value")
    if [[ -n $desc ]]; then displays+=("$value -- $desc"); else displays+=("$value"); fi
  done
  (( ${#values} )) && compadd -U -l -d displays -a values
  (( files )) && _files
  return 0
}

if [[ $funcstack[1] == _mgmt ]]; then
  _mgmt "$@"
else
  # sourced, not autoloaded from $fpath
  (( $+functions[compdef] )) || { autoload -Uz compinit && compinit; }
  compdef _mgmt mgmt
fi
