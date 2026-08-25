_cometline_dump_env() {
	local dir="${COMETLINE_ENV_DIR:-}"
	[[ -n "$dir" ]] || return 0
	(
		umask 077
		mkdir -p "$dir" || exit 1
		local tmp="$dir/environ.tmp"
		: >"$tmp" || exit 1
		local key
		if command -v compgen >/dev/null 2>&1; then
			while IFS= read -r key; do
				[[ -n "$key" ]] || continue
				printf '%s=%s\0' "$key" "${!key}" >>"$tmp" || exit 1
			done < <(compgen -e)
		else
			exit 1
		fi
		chmod 600 "$tmp" || true
		mv -f "$tmp" "$dir/environ" || exit 1
		chmod 600 "$dir/environ" || true
	)
}

case ";${PROMPT_COMMAND-};" in
	*";_cometline_dump_env;"*) ;;
	*)
		if [[ -n "${PROMPT_COMMAND-}" ]]; then
			PROMPT_COMMAND="_cometline_dump_env; ${PROMPT_COMMAND}"
		else
			PROMPT_COMMAND="_cometline_dump_env"
		fi
		;;
esac
