"""Apps Script collector: inventories automations and their integrations.

Scripts are discovered through Drive (mimeType
application/vnd.google-apps.script). The Apps Script API then reads each
project's source IN MEMORY ONLY to classify it. What is stored: counts
of scripts per Google service used and counts of scripts per external
host called. Script source code, project names and script IDs are never
persisted. Host extraction only covers literal URLs passed to
UrlFetchApp.fetch, so the result is a lower bound (dynamically built
URLs are not detected).
"""

import re
from collections import Counter

from demo.demo_data import DEMO_DATA
from .base import BaseCollector, Aggregate

SCRIPT_MIME = "application/vnd.google-apps.script"

# Google Apps Script services detected in source (in memory only).
SERVICE_PATTERNS = {
    "SpreadsheetApp": r"\bSpreadsheetApp\b",
    "DocumentApp": r"\bDocumentApp\b",
    "GmailApp/MailApp": r"\b(GmailApp|MailApp)\b",
    "CalendarApp": r"\bCalendarApp\b",
    "DriveApp": r"\bDriveApp\b",
    "FormApp": r"\bFormApp\b",
    "UrlFetchApp": r"\bUrlFetchApp\b",
    "Triggers": r"\bScriptApp\b[^;]*newTrigger",
}

FETCH_URL_RE = re.compile(
    r"""UrlFetchApp\s*\.\s*fetch\s*\(\s*['"`]https?://([^'"`/\s]+)"""
)
GOOGLE_HOSTS = ("google.com", "googleapis.com", "gstatic.com", "googleusercontent.com")


class AppsScriptCollector(BaseCollector):
    service = "apps_script"

    def collect(self) -> list[Aggregate]:
        from googleapiclient.discovery import build
        from googleapiclient.errors import HttpError

        from core.auth import load_credentials

        creds = load_credentials()
        if creds is None:
            raise RuntimeError("No token. Run 'python3 inventory.py auth' first.")

        drive = build("drive", "v3", credentials=creds)
        script = build("script", "v1", credentials=creds)

        # Discover script project IDs (IDs stay in memory, never stored).
        script_ids: list[str] = []
        page_token = None
        while True:
            response = drive.files().list(
                q=f"mimeType='{SCRIPT_MIME}'",
                fields="nextPageToken, files(id)",
                pageSize=1000,
                pageToken=page_token,
            ).execute()
            script_ids.extend(f["id"] for f in response.get("files", []))
            page_token = response.get("nextPageToken")
            if not page_token:
                break

        aggregates = [
            Aggregate(self.service, "total_script_projects", len(script_ids), "count")
        ]
        service_counts: Counter = Counter()
        host_counts: Counter = Counter()
        bound_scripts = 0
        unreadable = 0

        for script_id in script_ids:
            try:
                project = script.projects().get(scriptId=script_id).execute()
                if project.get("parentId"):
                    bound_scripts += 1
                content = script.projects().getContent(scriptId=script_id).execute()
                source = "\n".join(f.get("source", "") for f in content.get("files", []))
            except HttpError:
                unreadable += 1  # deleted or permission denied; skip silently
                continue

            for service_name, pattern in SERVICE_PATTERNS.items():
                if re.search(pattern, source):
                    service_counts[service_name] += 1
            for host in FETCH_URL_RE.findall(source):
                if not any(host == h or host.endswith("." + h) for h in GOOGLE_HOSTS):
                    host_counts[host] += 1

        aggregates.append(
            Aggregate(self.service, "bound_scripts", bound_scripts, "count"))
        aggregates.append(
            Aggregate(self.service, "standalone_scripts",
                      len(script_ids) - bound_scripts - unreadable, "count"))
        if unreadable:
            aggregates.append(
                Aggregate(self.service, "unreadable_projects", unreadable, "count"))
        for service_name, count in sorted(service_counts.items()):
            aggregates.append(
                Aggregate(self.service, "scripts_using_service", count,
                          "count", service_name))
        for host, count in sorted(host_counts.items()):
            aggregates.append(
                Aggregate(self.service, "scripts_calling_host", count,
                          "count", host))
        return aggregates

    def collect_demo(self) -> list[Aggregate]:
        return list(DEMO_DATA[self.service])
