"""Simulated example account for demo mode.

Plausible aggregates for a personal Google account (~4.9 GB total),
shaped exactly like what the real collectors will produce. No personal
data: metric names and aggregation keys only.
"""

from collectors.base import Aggregate

DEMO_DATA: dict[str, list[Aggregate]] = {
    "drive": [
        Aggregate("drive", "total_files", 1240, "count"),
        Aggregate("drive", "total_bytes", 4_890_000_000, "bytes"),
        # Breakdown by file type (detail = friendly type name)
        Aggregate("drive", "files_by_type", 180, "count", "Google Docs"),
        Aggregate("drive", "files_by_type", 95, "count", "Google Sheets"),
        Aggregate("drive", "files_by_type", 60, "count", "Google Slides"),
        Aggregate("drive", "files_by_type", 25, "count", "Google Forms"),
        Aggregate("drive", "files_by_type", 110, "count", "PDF"),
        Aggregate("drive", "files_by_type", 430, "count", "Images (binary)"),
        Aggregate("drive", "files_by_type", 120, "count", "Office/binaries"),
        Aggregate("drive", "files_by_type", 220, "count", "Folders"),
        Aggregate("drive", "files_by_type", 45, "count", "Other"),
        Aggregate("drive", "bytes_by_type", 900_000_000, "bytes", "Google Docs"),
        Aggregate("drive", "bytes_by_type", 400_000_000, "bytes", "Google Sheets"),
        Aggregate("drive", "bytes_by_type", 250_000_000, "bytes", "Google Slides"),
        Aggregate("drive", "bytes_by_type", 20_000_000, "bytes", "Google Forms"),
        Aggregate("drive", "bytes_by_type", 1_800_000_000, "bytes", "PDF"),
        Aggregate("drive", "bytes_by_type", 800_000_000, "bytes", "Images (binary)"),
        Aggregate("drive", "bytes_by_type", 520_000_000, "bytes", "Office/binaries"),
        Aggregate("drive", "bytes_by_type", 0, "bytes", "Folders"),
        Aggregate("drive", "bytes_by_type", 200_000_000, "bytes", "Other"),
        # Age and sharing (counts only, no identifiers)
        Aggregate("drive", "not_modified_12m", 610, "count"),
        Aggregate("drive", "shared_any", 210, "count"),
        Aggregate("drive", "shared_by_type", 95, "count", "anyone with the link"),
        Aggregate("drive", "shared_by_type", 80, "count", "specific people"),
        Aggregate("drive", "shared_by_type", 35, "count", "outside the domain"),
    ],
    "gmail": [
        Aggregate("gmail", "total_messages", 8320, "count"),
        Aggregate("gmail", "total_bytes", 2_140_000_000, "bytes"),
        Aggregate("gmail", "label_count", 24, "count"),
        Aggregate("gmail", "unread_over_1y", 1150, "count"),
    ],
    "calendar": [
        Aggregate("calendar", "calendar_count", 3, "count"),
        Aggregate("calendar", "events_past_year", 1180, "count"),
        Aggregate("calendar", "events_future", 145, "count"),
    ],
    "contacts": [
        Aggregate("contacts", "contact_count", 312, "count"),
        Aggregate("contacts", "group_count", 8, "count"),
    ],
    "photos": [
        Aggregate("photos", "photo_count", 1420, "count"),
        Aggregate("photos", "video_count", 86, "count"),
        Aggregate("photos", "total_bytes", 1_640_000_000, "bytes"),
    ],
    "tasks": [
        Aggregate("tasks", "list_count", 5, "count"),
        Aggregate("tasks", "open_task_count", 37, "count"),
    ],
    "apps_script": [
        Aggregate("apps_script", "total_script_projects", 12, "count"),
        Aggregate("apps_script", "standalone_scripts", 5, "count"),
        Aggregate("apps_script", "bound_scripts", 7, "count"),
        Aggregate("apps_script", "scripts_using_service", 9, "count", "DriveApp"),
        Aggregate("apps_script", "scripts_using_service", 8, "count", "SpreadsheetApp"),
        Aggregate("apps_script", "scripts_using_service", 5, "count", "GmailApp/MailApp"),
        Aggregate("apps_script", "scripts_using_service", 4, "count", "UrlFetchApp"),
        Aggregate("apps_script", "scripts_using_service", 3, "count", "CalendarApp"),
        Aggregate("apps_script", "scripts_using_service", 2, "count", "FormApp"),
        Aggregate("apps_script", "scripts_using_service", 4, "count", "Triggers"),
        Aggregate("apps_script", "scripts_calling_host", 2, "count", "api.slack.com"),
        Aggregate("apps_script", "scripts_calling_host", 1, "count", "api.notion.com"),
        Aggregate("apps_script", "scripts_calling_host", 1, "count", "hooks.zapier.com"),
        Aggregate("apps_script", "scripts_calling_host", 1, "count", "crm.example.com"),
    ],
}
