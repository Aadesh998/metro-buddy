# Source this to use the project-local Android toolchain in .local/toolchain (git-ignored).
#   source scripts/android-env.sh
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")/.." && pwd)"
export JAVA_HOME="$ROOT/.local/toolchain/jdk-17"
export ANDROID_HOME="$ROOT/.local/toolchain/android-sdk"
export GRADLE_USER_HOME="$ROOT/.local/toolchain/gradle-home"
export PATH="$JAVA_HOME/bin:$ANDROID_HOME/platform-tools:$PATH"
