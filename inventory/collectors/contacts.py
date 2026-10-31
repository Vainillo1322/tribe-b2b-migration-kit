"""Contacts collector (real): counts only, no names."""

from demo.demo_data import DEMO_DATA
from .base import BaseCollector, Aggregate


class ContactsCollector(BaseCollector):
    service = "contacts"

    def collect(self) -> list[Aggregate]:
        from googleapiclient.discovery import build

        from core.auth import load_credentials

        creds = load_credentials()
        if creds is None:
            raise RuntimeError("No token. Run 'python3 inventory.py auth' first.")
        people = build("people", "v1", credentials=creds)

        contact_count = 0
        page_token = None
        while True:
            response = people.people().connections().list(
                resourceName="people/me", pageSize=100,
                personFields="metadata",  # minimal field: counting only
                pageToken=page_token).execute()
            contact_count += len(response.get("connections", []))
            page_token = response.get("nextPageToken")
            if not page_token:
                break

        groups = people.contactGroups().list(pageSize=1000).execute()
        return [
            Aggregate(self.service, "contact_count", contact_count, "count"),
            Aggregate(self.service, "group_count",
                      len(groups.get("contactGroups", [])), "count"),
        ]

    def collect_demo(self) -> list[Aggregate]:
        return list(DEMO_DATA[self.service])
