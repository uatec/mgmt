# bash completion for mgmt. Load it with:  source <(mgmt completion bash)

_mgmt() {
  local line cur prefix
  local -a words out
  # Rebuild the words from the command line: bash splits "--format=json" in
  # three, but mgmt wants it whole.
  line=${COMP_LINE:0:COMP_POINT}
  read -ra words <<<"$line"
  [[ $line == *[[:space:]] ]] && words+=('')
  cur=${words[${#words[@]} - 1]}
  prefix=''
  [[ $cur == -*=* ]] && prefix=${cur%%=*}=

  mapfile -t out < <(command mgmt __complete "${words[@]:1}" 2>/dev/null)
  COMPREPLY=()
  local entry
  for entry in "${out[@]}"; do
    if [[ $entry == ':files' ]]; then
      compopt -o default
      continue
    fi
    entry=${entry%%$'\t'*}
    COMPREPLY+=("${entry#"$prefix"}")
  done
}

complete -F _mgmt mgmt
