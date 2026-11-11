#!/bin/sh
set -e

REPO="A7E28/babe"
API_URL="https://api.github.com/repos/${REPO}/releases/latest"
INSTALL_DIR="${HOME}/.local/bin"
TARGET_BIN="${INSTALL_DIR}/babe"

FORCE=0
for arg in "$@"; do
    case "$arg" in
        -f|--force) FORCE=1 ;;
    esac
done

OS_NAME="$(uname -s | tr '[:upper:]' '[:lower:]')"
if [ "$OS_NAME" != "linux" ]; then
    echo "[ERROR] Unsupported operating system: $OS_NAME (expected linux)" >&2
    exit 1
fi

RAW_ARCH="$(uname -m)"
case "$RAW_ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *)
        echo "[ERROR] Unsupported CPU architecture: $RAW_ARCH (supported: amd64, arm64)" >&2
        exit 1
        ;;
esac

fetch_url() {
    url="$1"
    dest="$2"
    if command -v curl >/dev/null 2>&1; then
        if [ -n "$dest" ]; then
            curl -fsSL -H "User-Agent: babe-installer" "$url" -o "$dest"
        else
            curl -fsSL -H "User-Agent: babe-installer" "$url"
        fi
    elif command -v wget >/dev/null 2>&1; then
        if [ -n "$dest" ]; then
            wget -q --header="User-Agent: babe-installer" -O "$dest" "$url"
        else
            wget -q --header="User-Agent: babe-installer" -O - "$url"
        fi
    else
        echo "[ERROR] Neither curl nor wget is installed." >&2
        exit 1
    fi
}

compute_sha256() {
    file="$1"
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$file" | awk '{print tolower($1)}'
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$file" | awk '{print tolower($1)}'
    else
        echo ""
    fi
}

echo "--- babe Installer (linux/${ARCH}) ---"
echo "[INFO] Querying latest release from GitHub..."

RELEASE_JSON="$(fetch_url "$API_URL" "")"
TAG="$(printf '%s\n' "$RELEASE_JSON" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name":[[:space:]]*"([^"]+)".*/\1/')"

if [ -z "$TAG" ]; then
    echo "[ERROR] Failed to resolve latest release tag from GitHub." >&2
    exit 1
fi

ASSET_NAME="babe-${TAG}-linux-${ARCH}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET_NAME}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${TAG}/checksums.txt"

EXPECTED_HASH=""
CHECKSUMS_CONTENT="$(fetch_url "$CHECKSUMS_URL" "" 2>/dev/null || true)"
if [ -n "$CHECKSUMS_CONTENT" ]; then
    EXPECTED_HASH="$(printf '%s\n' "$CHECKSUMS_CONTENT" | grep -F "$ASSET_NAME" | awk '{print tolower($1)}' | head -n1)"
fi

mkdir -p "$INSTALL_DIR"

NEED_DOWNLOAD=1
if [ -f "$TARGET_BIN" ] && [ "$FORCE" -eq 0 ] && [ -n "$EXPECTED_HASH" ]; then
    CURRENT_HASH="$(compute_sha256 "$TARGET_BIN")"
    if [ "$CURRENT_HASH" = "$EXPECTED_HASH" ]; then
        echo "[OK] babe ${TAG} is already installed and up to date at:"
        echo "     ${TARGET_BIN}"
        NEED_DOWNLOAD=0
    fi
fi

if [ "$NEED_DOWNLOAD" -eq 1 ]; then
    TEMP_FILE="${INSTALL_DIR}/.babe-install-$$.tmp"
    trap 'rm -f "$TEMP_FILE"' EXIT INT TERM

    echo "[INFO] Downloading ${ASSET_NAME} (${TAG})..."
    fetch_url "$DOWNLOAD_URL" "$TEMP_FILE"

    if [ -n "$EXPECTED_HASH" ]; then
        ACTUAL_HASH="$(compute_sha256 "$TEMP_FILE")"
        if [ -n "$ACTUAL_HASH" ] && [ "$ACTUAL_HASH" != "$EXPECTED_HASH" ]; then
            echo "[ERROR] SHA256 checksum mismatch! Expected ${EXPECTED_HASH}, got ${ACTUAL_HASH}" >&2
            rm -f "$TEMP_FILE"
            exit 1
        fi
        echo "[OK] Verified SHA256: ${ACTUAL_HASH}"
    fi

    chmod 0755 "$TEMP_FILE"
    mv -f "$TEMP_FILE" "$TARGET_BIN"
    trap - EXIT INT TERM
    echo "[OK] Installed babe ${TAG} to ${TARGET_BIN}"
fi

# Idempotent PATH registration matching internal/platform/path_linux.go
EXPORT_LINE='export PATH="$HOME/.local/bin:$PATH"'
UPDATED_RC=0

for rc in "${HOME}/.bashrc" "${HOME}/.zshrc"; do
    if [ -f "$rc" ]; then
        if grep -Fq ".local/bin" "$rc" 2>/dev/null; then
            UPDATED_RC=1
        else
            printf '\n# Added by babe\n%s\n' "$EXPORT_LINE" >> "$rc"
            echo "[OK] Added ~/.local/bin to ${rc}"
            UPDATED_RC=1
        fi
    fi
done

if [ "$UPDATED_RC" -eq 0 ]; then
    PROFILE="${HOME}/.profile"
    if ! grep -Fq ".local/bin" "$PROFILE" 2>/dev/null; then
        printf '\n# Added by babe\n%s\n' "$EXPORT_LINE" >> "$PROFILE"
        echo "[OK] Added ~/.local/bin to ${PROFILE}"
    fi
fi

case ":${PATH}:" in
    *":${INSTALL_DIR}:"*)
        echo "[OK] Session PATH already contains ${INSTALL_DIR}"
        ;;
    *)
        echo "[INFO] Restart your shell or run: export PATH=\"\$HOME/.local/bin:\$PATH\""
        ;;
esac

echo "[INFO] Done. Run 'babe' or 'babe doctor' from your terminal."
