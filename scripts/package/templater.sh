#!/usr/bin/env bash
# Replaces all {{VAR}} by the $VAR value in a template file and outputs it
# Use with -h to output all variables
# curtesy of https://github.com/lavoiesl/bash-templater/blob/master/templater.sh

if [[ ! -f "$1" ]]; then
    echo "Usage: VAR=value $0 template" >&2
    exit 1
fi

template="$1"
vars=$(grep -oE '\{\{\s*[A-Za-z0-9_]+\s*\}\}' "$template" | sort | uniq | sed -e 's/^{{//' -e 's/}}$//')

if [[ -z "$vars" ]]; then
    echo "Warning: No variable was found in $template, syntax is {{VAR}}" >&2
fi

var_value() {
    var="${1}"
    eval echo \$"${var}"
}

##
# Escape custom characters in a string
# Example: escape "ab'\c" '\' "'"   ===>  ab\'\\c
#
function escape_chars() {
    local content="${1}"
    shift

    for char in "$@"; do
        content="${content//${char}/\\${char}}"
    done

    echo "${content}"
}

function echo_var() {
    local var="${1}"
    local content="${2}"
    local escaped="$(escape_chars "${content}" "\\" '"')"

    echo "${var}=\"${escaped}\""
}

declare -a replaces
replaces=()

# Reads default values defined as {{VAR=value}} and delete those lines
# There are evaluated, so you can do {{PATH=$HOME}} or {{PATH=`pwd`}}
# You can even reference variables defined in the template before
defaults=$(grep -oE '^\{\{[A-Za-z0-9_]+=.+\}\}$' "${template}" | sed -e 's/^{{//' -e 's/}}$//')
IFS=$'\n'
for default in $defaults; do
    var=$(echo "${default}" | grep -oE "^[A-Za-z0-9_]+")
    current="$(var_value "${var}")"

    # Replace only if var is not set
    if [[ -n "$current" ]]; then
        eval "$(echo_var "${var}" "${current}")"
    else
        eval "${default}"
    fi

    # remove define line
    replaces+=("-e")
    replaces+=("/^{{${var}=/d")
    vars="${vars} ${var}"
done

vars="$(echo "${vars}" | tr " " "\n" | sort | uniq)"

if [[ "$2" = "-h" ]]; then
    for var in $vars; do
        value="$(var_value "${var}")"
        echo_var "${var}" "${value}"
    done
    exit 0
fi

# Replace all {{VAR}} by $VAR value
for var in $vars; do
    value="$(var_value "${var}")"
    if [[ -z "$value" ]]; then
        echo "Warning: $var is not defined and no default is set, replacing by empty" >&2
    fi

    # Escape slashes
    value="$(escape_chars "${value}" "\\" '/' ' ')";
    replaces+=("-e")
    replaces+=("s/{{\s*${var}\s*}}/${value}/g")
done

sed "${replaces[@]}" "${template}"
