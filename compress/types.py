import dataclasses

from bs4 import BeautifulSoup


@dataclasses.dataclass
class FileTagWrapper:
    tag: BeautifulSoup
    old_name: str
    new_name: str
