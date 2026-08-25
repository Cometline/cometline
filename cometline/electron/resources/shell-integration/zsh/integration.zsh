if [[ -o interactive ]]; then
	autoload -Uz add-zsh-hook 2>/dev/null || true
fi

_cometline_dump_env() {
	emulate -L zsh
	setopt no_nomatch
	local dir="${COMETLINE_ENV_DIR:-}"
	[[ -n "$dir" ]] || return 0
	(
		umask 077
		command mkdir -p "$dir" || exit 1
		local tmp="$dir/environ.tmp"
		: >|"$tmp" || exit 1
		local key
		for key in ${(k)parameters}; do
			[[ ${(Pt)key} == *export* ]] || continue
			printf '%s=%s\0' "$key" "${(P)key}" >>"$tmp" || exit 1
		done
		command chmod 600 "$tmp" || true
		command mv -f "$tmp" "$dir/environ" || exit 1
		command chmod 600 "$dir/environ" || true
	)
}

if [[ -o interactive ]] && typeset -f add-zsh-hook >/dev/null; then
	add-zsh-hook -d precmd _cometline_dump_env 2>/dev/null || true
	add-zsh-hook precmd _cometline_dump_env
fi
