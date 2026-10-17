// google-services.json is git-ignored (it carries a Firebase API key). Local builds read the
// file from this folder; EAS builds get it from a file variable:
//   eas env:create --name GOOGLE_SERVICES_JSON --type file --value ./google-services.json --visibility secret
module.exports = ({ config }) => ({
  ...config,
  android: { ...config.android, googleServicesFile: process.env.GOOGLE_SERVICES_JSON ?? './google-services.json' },
});
