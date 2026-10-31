"""Photos collector (real): photo/video counts only.

The Photos Library API does not expose file sizes, so volume here is a
count, not bytes. Skipped automatically if the scope was not granted.
"""

from demo.demo_data import DEMO_DATA
from .base import BaseCollector, Aggregate


class PhotosCollector(BaseCollector):
    service = "photos"

    def collect(self) -> list[Aggregate]:
        from googleapiclient.discovery import build

        from core.auth import load_credentials

        creds = load_credentials()
        if creds is None:
            raise RuntimeError("No token. Run 'python3 inventory.py auth' first.")
        photos = build("photoslibrary", "v1", credentials=creds,
                       static_discovery=False)

        photo_count = 0
        video_count = 0
        page_token = None
        while True:
            response = photos.mediaItems().list(
                pageSize=100, pageToken=page_token).execute()
            for item in response.get("mediaItems", []):
                mime = item.get("mimeType", "")
                if mime.startswith("video/"):
                    video_count += 1
                else:
                    photo_count += 1
            page_token = response.get("nextPageToken")
            if not page_token:
                break

        return [
            Aggregate(self.service, "photo_count", photo_count, "count"),
            Aggregate(self.service, "video_count", video_count, "count"),
        ]

    def collect_demo(self) -> list[Aggregate]:
        return list(DEMO_DATA[self.service])
