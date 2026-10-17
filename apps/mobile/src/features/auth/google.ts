import { GoogleSignin, isErrorWithCode, isSuccessResponse, statusCodes } from '@react-native-google-signin/google-signin';

const webClientId = process.env.EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID;

export const googleAvailable = !!webClientId;

let configured = false;
function configure() {
  if (configured || !webClientId) return;
  // The ID token is issued for the web client ID; the server checks it against that audience.
  GoogleSignin.configure({ webClientId, scopes: ['email'] });
  configured = true;
}

// Returns a Google ID token, or null if the user closed the account picker.
export async function googleIdToken(): Promise<string | null> {
  configure();
  try {
    await GoogleSignin.hasPlayServices({ showPlayServicesUpdateDialog: true });
    const res = await GoogleSignin.signIn();
    if (!isSuccessResponse(res)) return null;
    return res.data.idToken ?? null;
  } catch (e) {
    if (isErrorWithCode(e) && (e.code === statusCodes.SIGN_IN_CANCELLED || e.code === statusCodes.IN_PROGRESS)) return null;
    throw e;
  }
}

export async function googleSignOut() {
  if (!configured) return;
  await GoogleSignin.signOut().catch(() => {});
}
