"""OAuth desktop flow: consent, token storage and per-service access check.

Read-only scopes only. The token never leaves the local machine
(stored in data/token.json).
"""

import os
import json
from pathlib import Path

from google.auth.transport.requests import Request
from google.oauth2.credentials import Credentials
from google_auth_oauthlib.flow import InstalledAppFlow

BASE_DIR = Path(__file__).resolve().parent.parent
CREDENTIALS_DIR = BASE_DIR / "credentials"
CLIENT_SECRET_PATH = CREDENTIALS_DIR / "client_secret.json"
TOKEN_PATH = BASE_DIR / "data" / "token.json"

SERVICE_SCOPES = {
    "drive": "https://www.googleapis.com/auth/drive.metadata.readonly",
    "gmail": "https://www.googleapis.com/auth/gmail.metadata",
    "calendar": "https://www.googleapis.com/auth/calendar.readonly",
    "contacts": "https://www.googleapis.com/auth/contacts.readonly",
    "photos": "https://www.googleapis.com/auth/photoslibrary.readonly",
    "tasks": "https://www.googleapis.com/auth/tasks.readonly",
    "apps_script": "https://www.googleapis.com/auth/script.projects.readonly",
}


def _scopes_for(include_photos: bool = True) -> list[str]:
    return [
        scope for service, scope in SERVICE_SCOPES.items()
        if include_photos or service != "photos"
    ]


def _existing_scopes() -> set[str]:
    """Scopes already granted in a previous consent, to merge (never downgrade)."""
    if not TOKEN_PATH.exists():
        return set()
    try:
        creds = Credentials.from_authorized_user_file(str(TOKEN_PATH))
        return set(creds.scopes or [])
    except Exception:  # noqa: BLE001 - a corrupt token just means start over
        return set()


def check_setup() -> int:
    """Gate check for step 2.1: credentials file present and libraries OK."""
    print("Setup check")
    print("-" * 50)

    try:
        import google_auth_oauthlib  # noqa: F401
        print("  OAuth libraries      OK")
    except ImportError:
        print("  OAuth libraries      MISSING - run: pip install -r requirements.txt")
        return 1

    if CLIENT_SECRET_PATH.exists():
        print(f"  client_secret.json   OK ({CLIENT_SECRET_PATH})")
    else:
        print(f"  client_secret.json   MISSING - expected at: {CLIENT_SECRET_PATH}")
        print("                         Follow docs/credentials-setup.md to create it.")
        return 1

    if TOKEN_PATH.exists():
        print(f"  token.json           OK (consent already granted)")
        print("  Next step:           python3 inventory.py run")
    else:
        print("  token.json           not yet (run 'python3 inventory.py auth')")
    return 0


def load_credentials() -> Credentials | None:
    # Web mode: the Go server performs the browser OAuth flow and hands the
    # read-only access token over through the environment. No token file is
    # written, and the token lives only for this process.
    web_token = os.environ.get("INVENTORY_ACCESS_TOKEN", "").strip()
    if web_token:
        return Credentials(token=web_token, scopes=_scopes_for())
    if not TOKEN_PATH.exists():
        return None
    creds = Credentials.from_authorized_user_file(str(TOKEN_PATH),
                                                  _scopes_for())
    if creds.expired and creds.refresh_token:
        creds.refresh(Request())
        TOKEN_PATH.write_text(creds.to_json())
    return creds


def run_consent(only: list[str] | None = None,
                include_photos: bool = True) -> int:
    if not CLIENT_SECRET_PATH.exists():
        print(f"Missing {CLIENT_SECRET_PATH}. "
              "Follow docs/credentials-setup.md first.")
        return 1

    if only:
        unknown = [s for s in only if s not in SERVICE_SCOPES]
        if unknown:
            print(f"Unknown service(s): {', '.join(unknown)}. "
                  f"Valid: {', '.join(SERVICE_SCOPES)}")
            return 1
        requested = [SERVICE_SCOPES[s] for s in only]
    else:
        requested = _scopes_for(include_photos)

    # Never downgrade: merge requested scopes with anything already granted.
    scopes = sorted(set(requested) | _existing_scopes())

    flow = InstalledAppFlow.from_client_secrets_file(
        str(CLIENT_SECRET_PATH), scopes)
    creds = flow.run_local_server(port=0, prompt="consent")

    TOKEN_PATH.parent.mkdir(parents=True, exist_ok=True)
    TOKEN_PATH.write_text(creds.to_json())

    granted = set(creds.scopes or [])
    print("\nConsent granted. Per-service access:")
    print("-" * 50)
    for service, scope in SERVICE_SCOPES.items():
        if service == "photos" and not include_photos and not only:
            print(f"  {service:<12} SKIPPED (--no-photos)")
        elif scope in granted:
            print(f"  {service:<12} ACCESS GRANTED")
        else:
            print(f"  {service:<12} NOT GRANTED (check consent screen)")
    return 0
