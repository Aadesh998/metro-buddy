#!/usr/bin/env bash
# Builds a signed release APK into .local/builds/.
#   scripts/build-apk.sh [production|staging]
# Needs apps/mobile/google-services.json (from Firebase) and the signing key in .local/secrets/.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
source "$ROOT/scripts/android-env.sh"

case "${1:-production}" in
  production) API=https://api-production-ebaf.up.railway.app ;;
  staging) API=https://api-staging-b0d3.up.railway.app ;;
  *) echo "usage: $0 [production|staging]" >&2; exit 2 ;;
esac

SECRETS="$ROOT/.local/secrets"
[ -f "$ROOT/apps/mobile/google-services.json" ] || { echo "missing apps/mobile/google-services.json" >&2; exit 1; }
[ -f "$SECRETS/release.keystore" ] || { echo "missing $SECRETS/release.keystore" >&2; exit 1; }

cd "$ROOT/apps/mobile"
npx expo prebuild -p android --no-install
echo "sdk.dir=$ANDROID_HOME" > android/local.properties
cd android
EXPO_PUBLIC_API_URL="$API" NODE_ENV=production \
  MB_KEYSTORE="$SECRETS/release.keystore" MB_KEYSTORE_PASSWORD="$(cat "$SECRETS/release.keystore.password")" \
  ./gradlew --no-daemon assembleRelease -PreactNativeArchitectures=arm64-v8a

mkdir -p "$ROOT/.local/builds"
OUT="$ROOT/.local/builds/MetroBuddy-${1:-production}.apk"
cp app/build/outputs/apk/release/app-release.apk "$OUT"
echo "built $OUT"
