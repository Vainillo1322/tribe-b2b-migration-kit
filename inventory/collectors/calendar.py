"""Calendar collector (real): calendar count and event counts only."""

from demo.demo_data import DEMO_DATA
from .base import BaseCollector, Aggregate


class CalendarCollector(BaseCollector):
    service = "calendar"

    def collect(self) -> list[Aggregate]:
        from googleapiclient.discovery import build

        from core.auth import load_credentials

        creds = load_credentials()
        if creds is None:
            raise RuntimeError("No token. Run 'python3 inventory.py auth' first.")
        service = build("calendar", "v3", credentials=creds)

        calendars = service.calendarList().list().execute().get("items", [])
        aggregates = [
            Aggregate(self.service, "calendar_count", len(calendars), "count")]

        past_year = 0
        future = 0
        for calendar in calendars:
            past_year += self._count_events(service, calendar["id"], past=True)
            future += self._count_events(service, calendar["id"], past=False)
        aggregates.append(
            Aggregate(self.service, "events_past_year", past_year, "count"))
        aggregates.append(
            Aggregate(self.service, "events_future", future, "count"))
        return aggregates

    @staticmethod
    def _count_events(service, calendar_id: str, past: bool) -> int:
        """Count events in a time window (approximate, paged iteration)."""
        import datetime as dt
        from googleapiclient.errors import HttpError

        now = dt.datetime.now(dt.timezone.utc)
        time_min = (now - dt.timedelta(days=365)).isoformat() if past else now.isoformat()
        time_max = now.isoformat() if past else (now + dt.timedelta(days=365)).isoformat()

        count = 0
        page_token = None
        try:
            while True:
                response = service.events().list(
                    calendarId=calendar_id, timeMin=time_min, timeMax=time_max,
                    singleEvents=True, maxResults=2500,
                    pageToken=page_token).execute()
                count += len(response.get("items", []))
                page_token = response.get("nextPageToken")
                if not page_token:
                    break
        except HttpError:
            pass  # unreadable calendar; skip
        return count

    def collect_demo(self) -> list[Aggregate]:
        return list(DEMO_DATA[self.service])
