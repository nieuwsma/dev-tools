# Keep the existing theme, including its directory and Git information.
_machine_identity_color='@PROMPT_COLOR@'

# Safe to source again without duplicating the hostname.
if [[ -n ${_machine_identity_prefix:-} && $PROMPT == "$_machine_identity_prefix"* ]]; then
  PROMPT=${PROMPT#"$_machine_identity_prefix"}
fi
_machine_identity_prefix="%B%F{${_machine_identity_color}}%m%f%b "
PROMPT="${_machine_identity_prefix}${PROMPT}"

# tmux owns the project title while attached. Outside tmux, keep the
# hostname and current directory in the terminal title, even during commands.
DISABLE_AUTO_TITLE=true
machine_identity_title() {
  emulate -L zsh
  setopt nopromptsubst
  [[ -z ${TMUX:-} ]] || return 0
  [[ ${TERM:-dumb} != dumb ]] || return 0
  print -Pn '\e]2;%m · %~\a\e]1;%m · %~\a'
}
autoload -Uz add-zsh-hook
add-zsh-hook precmd machine_identity_title
