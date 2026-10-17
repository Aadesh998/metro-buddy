// Signs release builds with the key named by MB_KEYSTORE_* environment variables, so the
// generated android/ project never contains the key or its password. Without them, Gradle
// falls back to the template's debug key (fine for local testing, never for testers).
const { withAppBuildGradle } = require('expo/config-plugins');

const block = `
        release {
            if (System.getenv('MB_KEYSTORE')) {
                storeFile file(System.getenv('MB_KEYSTORE'))
                storePassword System.getenv('MB_KEYSTORE_PASSWORD')
                keyAlias System.getenv('MB_KEY_ALIAS') ?: 'metrobuddy'
                keyPassword System.getenv('MB_KEYSTORE_PASSWORD')
            }
        }`;

module.exports = function withReleaseSigning(config) {
  return withAppBuildGradle(config, (cfg) => {
    let g = cfg.modResults.contents;
    if (!g.includes("System.getenv('MB_KEYSTORE')")) {
      g = g.replace(/signingConfigs\s*\{/, (m) => m + block);
      g = g.replace(
        /(buildTypes\s*\{[\s\S]*?release\s*\{[\s\S]*?)signingConfig signingConfigs\.debug/,
        "$1signingConfig System.getenv('MB_KEYSTORE') ? signingConfigs.release : signingConfigs.debug",
      );
    }
    cfg.modResults.contents = g;
    return cfg;
  });
};
