#!/usr/bin/env bash
set -Eeuo pipefail

readonly HUI_REPOSITORY="GlareGlimmering/better-hy2"
readonly HUI_INSTALL_DIR="/usr/local/h-ui"
readonly HUI_STATE_DIR="/var/lib/h-ui"
readonly HUI_SERVICE="h-ui.service"
readonly HUI_BINARY_ASSET="h-ui-linux-amd64"
readonly HUI_CHECKSUM_ASSET="SHA256SUMS"

requested_version="latest"
use_prerelease=0
readonly panel_port="8000"
installer_work_dir=""

fail() {
  printf 'Better HY2 installer: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Install or upgrade Better HY2 on Linux AMD64 with systemd.

Usage:
  install.sh [version] [--version VERSION] [--prerelease] [--yes]

Without a version, the latest stable release is installed. Use --prerelease
to select the newest RC. Existing installations are upgraded in place. The
panel always listens on port 8000.
EOF
}

parse_args() {
  while (($#)); do
    case "$1" in
      --version)
        (($# >= 2)) || fail "--version requires a value"
        requested_version="$2"
        shift 2
        ;;
      --prerelease)
        use_prerelease=1
        shift
        ;;
      --port)
        fail "--port is no longer supported; the h-ui panel always uses port 8000"
        ;;
      --yes|-y)
        shift
        ;;
      --help|-h)
        usage
        exit 0
        ;;
      v[0-9]*)
        requested_version="$1"
        shift
        ;;
      *)
        fail "unknown argument: $1"
        ;;
    esac
  done
}

preflight_environment() {
  local effective_uid operating_system architecture systemd_dir
  if [[ "${HUI_INSTALLER_TEST:-0}" == "1" ]]; then
    effective_uid="${HUI_TEST_EUID:-$(id -u)}"
    operating_system="${HUI_TEST_OS:-$(uname -s)}"
    architecture="${HUI_TEST_ARCH:-$(uname -m)}"
    systemd_dir="${HUI_TEST_SYSTEMD_DIR:-/run/systemd/system}"
  else
    effective_uid="$(id -u)"
    operating_system="$(uname -s)"
    architecture="$(uname -m)"
    systemd_dir="/run/systemd/system"
  fi
  [[ "$effective_uid" == "0" ]] || fail "root privileges are required"
  [[ "$operating_system" == "Linux" ]] || fail "only Linux is supported"
  case "$architecture" in
    x86_64|amd64) ;;
    *) fail "unsupported architecture ${architecture}; D10 currently supports AMD64 only" ;;
  esac
  [[ -d "$systemd_dir" ]] || fail "systemd is not the active init system"
  if [[ "${HUI_INSTALLER_TEST:-0}" != "1" || "${HUI_TEST_SKIP_COMMANDS:-0}" != "1" ]]; then
    local command_name
    for command_name in curl sha256sum install mktemp systemctl sed grep awk head chmod mv dirname basename hostname sleep; do
      command -v "$command_name" >/dev/null 2>&1 || fail "required command is missing: $command_name"
    done
  fi
  [[ "$panel_port" =~ ^[0-9]+$ ]] && ((panel_port >= 1 && panel_port <= 65535)) || fail "invalid panel port: $panel_port"
}

allowed_version() {
  [[ "$1" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-custom\.(0|[1-9][0-9]*)(-rc\.(0|[1-9][0-9]*))?|-rc\.(0|[1-9][0-9]*))?$ ]]
}

version_is_older() {
  local left="$1" right="$2"
  local left_parts right_parts index
  left_parts="$(version_components "$left")" || return 2
  right_parts="$(version_components "$right")" || return 2
  read -r -a left_parts <<<"$left_parts"
  read -r -a right_parts <<<"$right_parts"
  for index in {0..5}; do
    if ((left_parts[index] < right_parts[index])); then return 0; fi
    if ((left_parts[index] > right_parts[index])); then return 1; fi
  done
  return 1
}

version_components() {
  local version="$1" release_rank=1 rc_number=0
  if [[ "$version" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
    printf '%d %d %d 2147483647 1 0\n' "$((10#${BASH_REMATCH[1]}))" "$((10#${BASH_REMATCH[2]}))" "$((10#${BASH_REMATCH[3]}))"
    return
  fi
  if [[ "$version" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)-rc\.([0-9]+)$ ]]; then
    printf '%d %d %d 2147483647 0 %d\n' "$((10#${BASH_REMATCH[1]}))" "$((10#${BASH_REMATCH[2]}))" "$((10#${BASH_REMATCH[3]}))" "$((10#${BASH_REMATCH[4]}))"
    return
  fi
  [[ "$version" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)-custom\.([0-9]+)(-rc\.([0-9]+))?$ ]] || return 1
  if [[ -n "${BASH_REMATCH[6]:-}" ]]; then
    release_rank=0
    rc_number="${BASH_REMATCH[6]}"
  fi
  printf '%d %d %d %d %d %d\n' \
    "$((10#${BASH_REMATCH[1]}))" "$((10#${BASH_REMATCH[2]}))" \
    "$((10#${BASH_REMATCH[3]}))" "$((10#${BASH_REMATCH[4]}))" \
    "$release_rank" "$((10#$rc_number))"
}

select_latest_prerelease() {
  local releases_json="$1" candidate="" tag
  while IFS= read -r tag; do
    if [[ -z "$candidate" ]] || version_is_older "$candidate" "$tag"; then
      candidate="$tag"
    fi
  done < <(
    printf '%s\n' "$releases_json" \
      | grep -oE '"tag_name"[[:space:]]*:[[:space:]]*"[^"]+"' \
      | sed -E 's/^.*"([^"]+)"$/\1/' \
      | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+(-custom\.[0-9]+)?-rc\.[0-9]+$' || true
  )
  printf '%s\n' "$candidate"
}

ensure_no_automatic_downgrade() {
  local target="$1" current="$2"
  if [[ "$current" == *-rc.* && "$target" != *-rc.* ]] && allowed_version "$current" && version_is_older "$target" "$current"; then
    fail "refusing prerelease-to-stable downgrade; restore the matching pre-upgrade snapshot"
  fi
  if [[ "$requested_version" == "latest" ]] && allowed_version "$current" && version_is_older "$target" "$current"; then
    fail "refusing automatic downgrade from ${current} to ${target}"
  fi
}

resolve_version() {
  local releases_json tag
  if [[ "$requested_version" != "latest" ]]; then
    allowed_version "$requested_version" || fail "invalid h-ui version: $requested_version"
    printf '%s\n' "$requested_version"
    return
  fi
  if ((use_prerelease)); then
    releases_json="$(curl -fsSL --retry 3 "https://api.github.com/repos/${HUI_REPOSITORY}/releases?per_page=30")" || fail "could not query prereleases"
    tag="$(select_latest_prerelease "$releases_json")"
  else
    releases_json="$(curl -fsSL --retry 3 "https://api.github.com/repos/${HUI_REPOSITORY}/releases/latest")" || fail "could not query the latest stable release"
    if grep -Eq '"(prerelease|draft)"[[:space:]]*:[[:space:]]*true' <<<"$releases_json"; then
      fail "stable channel returned a draft or prerelease"
    fi
    tag="$(printf '%s\n' "$releases_json" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1 || true)"
  fi
  [[ -n "$tag" ]] && allowed_version "$tag" || fail "GitHub did not return an allowed release version"
  if ((!use_prerelease)) && [[ "$tag" == *-rc.* ]]; then fail "stable channel returned a prerelease tag"; fi
  printf '%s\n' "$tag"
}

download_and_verify() {
  local version="$1" destination="$2" sums="$3"
  local base="https://github.com/${HUI_REPOSITORY}/releases/download/${version}"
  curl -fL --retry 3 --output "$destination" "${base}/${HUI_BINARY_ASSET}" || fail "failed to download ${HUI_BINARY_ASSET}"
  curl -fL --retry 3 --output "$sums" "${base}/${HUI_CHECKSUM_ASSET}" || fail "failed to download ${HUI_CHECKSUM_ASSET}"
  (
    cd "$(dirname "$destination")"
    grep -E "^[[:xdigit:]]{64}[[:space:]]+\*?${HUI_BINARY_ASSET}$" "$(basename "$sums")" > expected.sha256 || fail "SHA256SUMS does not contain ${HUI_BINARY_ASSET}"
    sha256sum --check expected.sha256
  ) || fail "h-ui SHA-256 verification failed"
  chmod 0755 "$destination" || fail "could not make the verified candidate executable"
  local reported
  reported="$("$destination" version 2>&1)" || fail "verified candidate could not execute"
  [[ "$reported" == "Better HY2 version ${version}" || "$reported" == "h-ui version ${version}" || "$reported" == "$version" ]] || fail "candidate reports '${reported}', expected ${version}"
}

install_systemd_unit() {
  local temporary
  temporary="$(mktemp /etc/systemd/system/.h-ui.service.XXXXXX)"
  cat >"$temporary" <<EOF
[Unit]
Description=Better HY2
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=${HUI_INSTALL_DIR}
Environment=HUI_DATA=${HUI_INSTALL_DIR}/
Environment=GIN_MODE=release
ExecStart=${HUI_INSTALL_DIR}/h-ui -p ${panel_port}
Restart=on-failure
RestartSec=3s
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
EOF
  chmod 0644 "$temporary"
  mv -f "$temporary" "/etc/systemd/system/${HUI_SERVICE}"
  systemctl daemon-reload
  systemctl enable "$HUI_SERVICE" >/dev/null
}

extract_frontend_entry_asset() {
  local html="$1"
  local double_quoted='<script[^>]*[[:space:]]src="([^"]+)"'
  local single_quoted="<script[^>]*[[:space:]]src='([^']+)'"

  if [[ "$html" =~ $double_quoted ]] || [[ "$html" =~ $single_quoted ]]; then
    printf '%s\n' "${BASH_REMATCH[1]}"
    return 0
  fi
  return 1
}

panel_health_check() {
  local base="http://127.0.0.1:${panel_port}/" html asset attempt
  for attempt in {1..20}; do
    if html="$(curl -fsS --max-time 3 "$base" 2>/dev/null)" && grep -Fq '<div id="app"></div>' <<<"$html"; then
      asset="$(extract_frontend_entry_asset "$html" || true)"
      if [[ -n "$asset" ]] && curl -fsS --max-time 3 "http://127.0.0.1:${panel_port}/${asset#/}" >/dev/null; then
        return 0
      fi
    fi
    sleep 1
  done
  return 1
}

fresh_install() {
  local version="$1" candidate="$2"
  install -d -m 0755 "${HUI_INSTALL_DIR}" "${HUI_INSTALL_DIR}/bin" "${HUI_INSTALL_DIR}/data" "${HUI_INSTALL_DIR}/logs" "${HUI_INSTALL_DIR}/export"
  install -d -m 0750 "${HUI_STATE_DIR}" "${HUI_STATE_DIR}/backups" "${HUI_STATE_DIR}/xray" "${HUI_STATE_DIR}/xray/certs"
  install -d -m 0755 /var/lib/hui

  (
    cd "$HUI_INSTALL_DIR"
    HUI_DATA="${HUI_INSTALL_DIR}/" "$candidate" install-hysteria2
  ) || fail "custom Hysteria2 installation failed; no official-kernel fallback was attempted"

  printf 'Creating the initial random administrator credentials (shown once):\n'
  (cd "$HUI_INSTALL_DIR" && HUI_DATA="${HUI_INSTALL_DIR}/" "$candidate" reset) || fail "initial administrator setup failed"

  local staged="${HUI_INSTALL_DIR}/.h-ui-install-${version}"
  install -m 0755 "$candidate" "$staged"
  mv -f "$staged" "${HUI_INSTALL_DIR}/h-ui"
  install_systemd_unit
  systemctl restart "$HUI_SERVICE" || fail "h-ui service failed to start"
  systemctl is-active --quiet "$HUI_SERVICE" || fail "h-ui service is not active"
  panel_health_check || fail "h-ui started but the panel or its frontend entry asset failed the HTTP health check"
}

validate_cli_link() {
  local link="/usr/local/bin/hui" target="${HUI_INSTALL_DIR}/h-ui"
  if [[ -e "$link" || -L "$link" ]]; then
    [[ -L "$link" && "$(readlink "$link")" == "$target" ]] || fail "refusing to overwrite an unrelated hui command: $link"
  fi
}

install_cli_link() {
  local link="/usr/local/bin/hui" target="${HUI_INSTALL_DIR}/h-ui"
  validate_cli_link
  if [[ -L "$link" ]]; then return; fi
  install -d -m 0755 /usr/local/bin
  ln -s "$target" "$link" || fail "could not create hui command"
}

upgrade_existing() {
  local version="$1" candidate="$2"
  local -a arguments=(upgrade "$version" --yes)
  if ((use_prerelease)); then arguments+=(--prerelease); fi
  "$candidate" "${arguments[@]}"
}

valid_existing_installation() {
  local reported
  [[ -x "${HUI_INSTALL_DIR}/h-ui" ]] || return 1
  reported="$("${HUI_INSTALL_DIR}/h-ui" version 2>/dev/null)" || return 1
  [[ "$reported" == "h-ui version "* || "$reported" == "Better HY2 version "* ]] || return 1
  systemctl cat --no-pager "$HUI_SERVICE" >/dev/null 2>&1
}

cleanup_work_dir() {
  if [[ -n "${installer_work_dir:-}" ]]; then
    rm -rf -- "$installer_work_dir"
  fi
}

create_installer_work_dir() {
  installer_work_dir="$(mktemp -d)"
  trap cleanup_work_dir EXIT
}

main() {
  parse_args "$@"
  preflight_environment
  validate_cli_link
  local target work_dir candidate sums existing=0
  target="$(resolve_version)"
  if valid_existing_installation; then
    existing=1
  elif [[ -e "${HUI_INSTALL_DIR}/h-ui" || -d "${HUI_INSTALL_DIR}/bin" || -f "/etc/systemd/system/${HUI_SERVICE}" ]]; then
    printf 'Incomplete h-ui installation detected; safely resuming fresh initialization.\n'
  fi

  if ((existing)); then
    local current_version
    current_version="$("${HUI_INSTALL_DIR}/h-ui" version 2>/dev/null)"
    current_version="${current_version#h-ui version }"
    current_version="${current_version#Better HY2 version }"
    ensure_no_automatic_downgrade "$target" "$current_version"
  fi

  create_installer_work_dir
  work_dir="$installer_work_dir"
  candidate="${work_dir}/${HUI_BINARY_ASSET}"
  sums="${work_dir}/${HUI_CHECKSUM_ASSET}"
  download_and_verify "$target" "$candidate" "$sums"

  if ((existing)); then
    upgrade_existing "$target" "$candidate"
  else
    fresh_install "$target" "$candidate"
  fi
  install_cli_link

  local hui_version hy2_version service_state panel_summary
  hui_version="$("${HUI_INSTALL_DIR}/h-ui" version 2>/dev/null || true)"
  hy2_version="$("${HUI_INSTALL_DIR}/bin/hysteria2" version 2>/dev/null | head -n 1 || true)"
  service_state="$(systemctl is-active "$HUI_SERVICE" 2>/dev/null || true)"
  panel_summary="http://127.0.0.1:${panel_port}"
  cat <<EOF

Better HY2 installed successfully

Panel:   ${panel_summary}
h-ui:    ${hui_version:-unknown}
HY2:     ${hy2_version:-unknown} (installed; activation follows the saved panel configuration)
Service: ${service_state:-unknown}

Future upgrades:
  sudo hui update
  sudo /usr/local/h-ui/h-ui upgrade
EOF
}

if [[ "${HUI_INSTALLER_TEST:-0}" != "1" ]]; then
  main "$@"
fi
