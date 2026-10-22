# fish completion for mgmt. Load it with:  mgmt completion fish | source

function __mgmt_complete
    set -l words (commandline -opc)
    set -e words[1]
    set -l out (command mgmt __complete $words (commandline -ct) 2>/dev/null)
    if test "$out[-1]" = ':files'
        set -e out[-1]
        __fish_complete_path (commandline -ct)
    end
    printf '%s\n' $out
end

complete -c mgmt -f -a '(__mgmt_complete)'
