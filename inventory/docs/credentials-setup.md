# Google Cloud Console: credential setup guide (step 2.1)

Exact steps to create the OAuth credentials the inventory tool needs.
Estimated time: 5-10 minutes. Everything happens in your own browser;
no data leaves your machine afterwards.

## 1. Create the project

1. Open https://console.cloud.google.com and sign in with the Google
   account you will test with.
2. Top bar: project selector -> **New project**.
3. Name it `workspace-inventory` (or any name) -> **Create**.
4. Select the new project in the top bar.

## 2. Enable the seven APIs

Menu -> **APIs & Services** -> **Library**. Search and **Enable** each:

1. Google Drive API
2. Gmail API
3. Google Calendar API
4. People API (contacts)
5. Photos Library API
6. Tasks API
7. Apps Script API

## 3. Configure the OAuth consent screen

**APIs & Services** -> **OAuth consent screen**:

1. User type: **External** -> Create.
2. App name: `Workspace Inventory` (only you see this).
3. User support email and developer contact: your own email.
4. Scopes: you can skip adding scopes manually; the tool requests them
   at consent time. Save.
5. Test users: **Add users** -> add the SAME account you are testing
   with. This is what allows the consent to work without verification.
6. Save. Leave the app in "Testing" status.

Note on Photos: `photoslibrary.readonly` is a restricted scope. With a
testing app and yourself as test user it usually works, but if Google
blocks it, no problem: run the tool with `--no-photos` and Photos will
be reported as "not inventoriable in this run".

## 4. Create the OAuth client (desktop app)

**APIs & Services** -> **Credentials** -> **Create credentials** ->
**OAuth client ID**:

1. Application type: **Desktop app**.
2. Name: `inventory-desktop`.
3. **Create**, then **Download JSON**.

Save the downloaded file in the project as:

```
google-inventory-mvp/credentials/client_secret.json
```

(create the `credentials/` folder next to `inventory.py`; the file name
must stay `client_secret.json`).

## 5. Create a virtual environment, install dependencies and check

In the project folder, create and activate a virtual environment:

```
# macOS / Linux
python3 -m venv .venv
source .venv/bin/activate

# Windows (PowerShell)
python -m venv .venv
.venv\Scripts\Activate.ps1
```

With the environment activated (your prompt shows `(.venv)`):

```
pip install -r requirements.txt
python3 inventory.py auth --check
```

Expected output: `client_secret.json OK`. When you see it, step 2.1 is
done and you can run the consent flow:

```
python3 inventory.py auth
```

The browser opens, you review the read-only scopes and grant access.
The tool then prints the per-service access status (step 2.2 gate).

## Security notes

- The `client_secret.json` and `data/token.json` files are for your
  machine only. Do not commit them to git or share them.
- The tool requests read-only scopes exclusively (`readonly` /
  `metadata`). It can never modify or delete anything.
