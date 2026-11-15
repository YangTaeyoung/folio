# Optional Google sign-in

Google sign-in is disabled by default. Each on-premises installation owner creates and manages their own Google OAuth client. Folio's password login and core publishing features work without Google or an internet connection; Google sign-in requires access to Google's authentication services.

## Connect an installation

1. Sign in with an existing Folio administrator account and open **조직 설정 → Google 로그인**.
2. In your own Google Cloud project, configure **Google Auth Platform → Branding** and **Audience** for your users. If the app is in testing, add the accounts that will test sign-in. Choose an audience appropriate for the installation; a Workspace-only audience will not admit personal Gmail users.
3. Under **Clients**, create an OAuth client with application type **Web application**. Use a client owned by this installation's operator rather than credentials bundled with Folio or another installation.
4. Add the exact **승인된 리디렉션 URI** shown by Folio to the client's **Authorized redirect URIs**. For an HTTPS installation with `AppURL=https://folio.example.com`, this is:

   ```text
   https://folio.example.com/api/auth/google/callback
   ```

   The scheme, hostname, port, and callback path must match the address shown by Folio. Register the management address, not an artifact content hostname. Update the Google client if the management address changes.
5. Enter the **Client ID** and **Client Secret** in Folio. Optionally enter a comma-separated list of allowed Google Workspace domains. Enable Google login and save.
6. Precreate each user's account in **구성원** with the exact email of their Google account. To create a Google-only account, choose **Google 계정** as the login method. This option appears when Google sign-in is enabled. Test sign-in before relying on it for access.

The saved client secret is encrypted by the server and is never returned to the settings page. Leave the secret field blank to keep it; enter a new value to replace it. Do not put secrets in source control, screenshots, logs, or public documentation.

## Accounts and restrictions

- Only active accounts already created in this Folio installation can sign in. Google sign-in does not register new users or grant administrator privileges. Existing Folio roles remain authoritative.
- Folio requests only the `openid` and `email` identity scopes. It does not request access to Drive, Gmail messages, Calendar, or other Google application data. The precreated Folio name is retained.
- Sign-in requires a verified email for which Google is authoritative: a Gmail address, or a Google Workspace account carrying a verified hosted-domain (`hd`) claim. A Google account that uses an unrelated third-party email address is not sufficient.
- An empty domain list admits eligible Gmail and Workspace accounts, subject to the precreated-account rule. A nonempty list requires an exact Google Workspace hosted-domain claim. Enter domains without `@`, URL schemes, or wildcards. Adding `gmail.com` does not admit Gmail accounts under this Workspace restriction.
- A password account can also sign in through Google after its email is verified by Google. Its existing password remains available and its required password-change gate still applies. A Google-only account has no Folio password or temporary password; manage its password through Google. Folio hides password reset and password change for Google-only accounts.
- Keep an active password-based administrator as a local fallback. If Google is unavailable or the provider is disabled, Google-only users cannot sign in. Review these accounts before disabling the provider or tightening domain restrictions. Folio prevents changes that would leave no administrator with an available login method.

## Official setup references

- [Google OpenID Connect: OAuth client, redirect URI, consent, and token claims](https://developers.google.com/identity/openid-connect/openid-connect)
- [Google OAuth web application client creation](https://developers.google.com/workspace/guides/create-credentials#web_application)

The server-side implementation is in `internal/server/google_auth.go`; account provisioning and password restrictions are in `internal/server/auth.go`. Deployment credentials and administrator identities belong to the installation owner and are deliberately absent from this guide.
