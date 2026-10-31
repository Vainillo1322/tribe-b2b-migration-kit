"""Gmail collector (real): totals and labels only, never message content."""

from demo.demo_data import DEMO_DATA
from .base import BaseCollector, Aggregate


class GmailCollector(BaseCollector):
    service = "gmail"

    def collect(self) -> list[Aggregate]:
        from googleapiclient.discovery import build

        from core.auth import load_credentials

        creds = load_credentials()
        if creds is None:
            raise RuntimeError("No token. Run 'python3 inventory.py auth' first.")
        gmail = build("gmail", "v1", credentials=creds)

        profile = gmail.users().getProfile(userId="me").execute()
        aggregates = [
            Aggregate(self.service, "total_messages",
                      profile.get("messagesTotal", 0), "count"),
            Aggregate(self.service, "total_threads",
                      profile.get("threadsTotal", 0), "count"),
        ]
        quota = profile.get("storageQuota", {})
        if quota.get("usage") is not None:
            aggregates.append(
                Aggregate(self.service, "mailbox_bytes", quota["usage"], "bytes"))

        labels = gmail.users().labels().list(userId="me").execute()
        aggregates.append(
            Aggregate(self.service, "label_count",
                      len(labels.get("labels", [])), "count"))

        # Approximate: Google returns resultSizeEstimate for search counts.
        old_unread = gmail.users().messages().list(
            userId="me", q="is:unread older_than:1y",
            maxResults=1).execute()
        aggregates.append(
            Aggregate(self.service, "unread_over_1y",
                      old_unread.get("resultSizeEstimate", 0), "count"))
        return aggregates

    def collect_demo(self) -> list[Aggregate]:
        return list(DEMO_DATA[self.service])
