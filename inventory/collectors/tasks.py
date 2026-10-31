"""Tasks collector (real): list and open-task counts only, no titles."""

from demo.demo_data import DEMO_DATA
from .base import BaseCollector, Aggregate


class TasksCollector(BaseCollector):
    service = "tasks"

    def collect(self) -> list[Aggregate]:
        from googleapiclient.discovery import build

        from core.auth import load_credentials

        creds = load_credentials()
        if creds is None:
            raise RuntimeError("No token. Run 'python3 inventory.py auth' first.")
        tasks = build("tasks", "v1", credentials=creds)

        lists = tasks.tasklists().list(maxResults=100).execute().get("items", [])
        open_tasks = 0
        for task_list in lists:
            page_token = None
            while True:
                response = tasks.tasks().list(
                    tasklist=task_list["id"], showCompleted=False,
                    showHidden=False, maxResults=100,
                    pageToken=page_token).execute()
                open_tasks += len(response.get("items", []))
                page_token = response.get("nextPageToken")
                if not page_token:
                    break

        return [
            Aggregate(self.service, "list_count", len(lists), "count"),
            Aggregate(self.service, "open_task_count", open_tasks, "count"),
        ]

    def collect_demo(self) -> list[Aggregate]:
        return list(DEMO_DATA[self.service])
