"""All collectors, in execution order."""

from .apps_script import AppsScriptCollector
from .base import Aggregate, BaseCollector
from .calendar import CalendarCollector
from .contacts import ContactsCollector
from .drive import DriveCollector
from .gmail import GmailCollector
from .photos import PhotosCollector
from .tasks import TasksCollector

ALL_COLLECTORS = [
    DriveCollector,
    GmailCollector,
    CalendarCollector,
    ContactsCollector,
    PhotosCollector,
    TasksCollector,
    AppsScriptCollector,
]

__all__ = [
    "Aggregate",
    "BaseCollector",
    "ALL_COLLECTORS",
    "DriveCollector",
    "GmailCollector",
    "CalendarCollector",
    "ContactsCollector",
    "PhotosCollector",
    "TasksCollector",
    "AppsScriptCollector",
]
