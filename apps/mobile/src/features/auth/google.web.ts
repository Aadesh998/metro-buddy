// Google sign-in runs through the native SDK; the web build (used for testing) hides it.
export const googleAvailable = false;

export async function googleIdToken(): Promise<string | null> {
  return null;
}

export async function googleSignOut() {}
