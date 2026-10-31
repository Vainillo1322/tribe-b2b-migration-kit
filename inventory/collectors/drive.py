"""Drive collector (real): metadata only, aggregates only.

Reads mimeType, size, modifiedTime and the shared flag. Never reads
names or content. Detailed shared-by-type listing is left for a later
phase (it needs one extra API call per file).
"""

from datetime import datetime, timedelta, timezone

from demo.demo_data import DEMO_DATA
from .base import BaseCollector, Aggregate

MIME_NAMES = {
    "application/vnd.google-apps.document": "Google Docs",
    "application/vnd.google-apps.spreadsheet": "Google Sheets",
    "application/vnd.google-apps.presentation": "Google Slides",
    "application/vnd.google-apps.form": "Google Forms",
    "application/vnd.google-apps.folder": "Folders",
    "application/vnd.google-apps.script": "Apps Script projects",
    "application/pdf": "PDF",
}
IMAGE_PREFIX = "image/"
SCRIPT_PREFIX = "application/vnd.google-apps.script"


def _friendly_type(mime: str) -> str:
    if mime in MIME_NAMES:
        return MIME_NAMES[mime]
    if mime.startswith(IMAGE_PREFIX):
        return "Images (binary)"
    return "Other"


class DriveCollector(BaseCollector):
    service = "drive"

    def collect(self) -> list[Aggregate]:
        from googleapiclient.discovery import build

        from core.auth import load_credentials

        creds = load_credentials()
        if creds is None:
            raise RuntimeError("No token. Run 'python3 inventory.py auth' first.")
        drive = build("drive", "v3", credentials=creds)

        cutoff = (datetime.now(timezone.utc) - timedelta(days=365)).strftime("%Y-%m-%dT%H:%M:%SZ")

        total_files = 0
        total_bytes = 0
        native_no_size = 0
        not_modified_12m = 0
        shared_any = 0
        by_type_files: dict[str, int] = {}
        by_type_bytes: dict[str, int] = {}

        page_token = None
        while True:
            response = drive.files().list(
                q="trashed = false",
                fields="nextPageToken, files(mimeType, size, modifiedTime, shared)",
                pageSize=1000,
                pageToken=page_token,
            ).execute()
            for f in response.get("files", []):
                total_files += 1
                mime = f.get("mimeType", "")
                ftype = _friendly_type(mime)
                by_type_files[ftype] = by_type_files.get(ftype, 0) + 1

                size = f.get("size")
                if size is None:
                    native_no_size += 1  # native Google formats have no size
                else:
                    size = int(size)
                    total_bytes += size
                    by_type_bytes[ftype] = by_type_bytes.get(ftype, 0) + size

                modified = f.get("modifiedTime")
                if modified and modified < cutoff:
                    not_modified_12m += 1
                if f.get("shared"):
                    shared_any += 1

            page_token = response.get("nextPageToken")
            if not page_token:
                break

        aggregates = [
            Aggregate(self.service, "total_files", total_files, "count"),
            Aggregate(self.service, "total_bytes", total_bytes, "bytes"),
            Aggregate(self.service, "native_files_no_size", native_no_size, "count"),
            Aggregate(self.service, "not_modified_12m", not_modified_12m, "count"),
            Aggregate(self.service, "shared_any", shared_any, "count"),
        ]
        for ftype, count in sorted(by_type_files.items()):
            aggregates.append(
                Aggregate(self.service, "files_by_type", count, "count", ftype))
        for ftype, size in sorted(by_type_bytes.items()):
            aggregates.append(
                Aggregate(self.service, "bytes_by_type", size, "bytes", ftype))
        return aggregates

    def collect_demo(self) -> list[Aggregate]:
        return list(DEMO_DATA[self.service])
