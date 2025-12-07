import abc
import hashlib
import logging
import math
import os
import subprocess
from asyncio import Queue
from enum import Enum
from io import BytesIO
from zipfile import ZipFile

from PIL import Image
from bs4 import BeautifulSoup

from data import LogEntrySchema

logger = logging.getLogger(__name__)


class FileType(Enum):
    IMAGE = 'image'
    VIDEO = 'video'
    AUDIO = 'audio'


class CompressorBase(abc.ABC):
    def __init__(self, source: ZipFile, dest: ZipFile, soup: BeautifulSoup, queue: Queue):
        self.source = source
        self.dest = dest
        self.soup = soup
        self.queue = queue

    @staticmethod
    def optimize_image(ext: str, content: bytes) -> tuple[str, bytes]:
        ext = ext.lower()

        with Image.open(BytesIO(content)) as img:
            w, h = img.size
            if w > 800 or h > 800:
                scale = min(800 / w, 800 / h)
                img = img.resize((math.floor(w * scale), math.floor(h * scale)))

            fmt = ext.strip('.').upper()
            if ext in ('.jpe', '.jpg', '.jpeg', '.jfif', '.webp') or (ext == '.png' and img.mode in ('RGB', 'P')):
                ext = '.webp'
                fmt = 'WEBP'

            buff = BytesIO()
            img.save(buff, fmt, optimize=True)
            buff.seek(0)
        return ext, buff.read()

    @staticmethod
    def optimize_video(ext: str, content: bytes) -> tuple[str, bytes]:
        cmd = [
            'ffmpeg',
            '-i', 'pipe:',
            '-f', 'mp4',
            '-movflags', 'isml+frag_keyframe+empty_moov+default_base_moof',
            '-c:v', 'libx264',
            '-c:a', 'aac',
            '-b:a', '64k',
            '-preset', 'slow',
            '-profile:v', 'main',
            '-vf', 'scale=1024:664:force_original_aspect_ratio=decrease,setsar=1,scale=trunc(iw/2)*2:trunc(ih/2)*2',
            '-crf', '28',
            '-loglevel', 'error',
            'pipe:'
        ]
        proc = subprocess.Popen(cmd, stdout=subprocess.PIPE, stdin=subprocess.PIPE, stderr=subprocess.PIPE, bufsize=10 ** 8)
        try:
            (out, err) = proc.communicate(input=content, timeout=300)
            if proc.returncode == 0 and out:
                return '.mp4', out
        except Exception as e:
            logger.exception(e, "Failed to convert video")
            proc.kill()

        return ext, content

    @staticmethod
    def optimize_audio(ext: str, content: bytes) -> tuple[str, bytes]:
        cmd = [
            'ffmpeg',
            '-i', 'pipe:',
            '-f', 'mp3',
            '-b:a', '64k',
            '-loglevel', 'error',
            'pipe:'
        ]
        proc = subprocess.Popen(cmd, stdout=subprocess.PIPE, stdin=subprocess.PIPE, stderr=subprocess.PIPE, bufsize=10 ** 8)
        try:
            (out, err) = proc.communicate(input=content, timeout=300)
            if proc.returncode == 0 and out:
                return '.mp3', out
        except Exception as e:
            logger.exception(e, "Failed to convert audio")
            proc.kill()

        return ext, content

    @abc.abstractmethod
    def get_optimizable_items_count(self) -> int:
        ...

    @abc.abstractmethod
    def get_file_content(self, file_type: FileType, filename: str) -> bytes | None:
        ...

    @abc.abstractmethod
    def compress(self) -> None:
        ...

    def optimize_file(self, file_type: FileType, filename: str) -> str | None:
        name, ext = os.path.splitext(filename)
        if content := self.get_file_content(file_type, filename):
            new_ext, new_content = ext, content

            try:
                match file_type:
                    case FileType.IMAGE:
                        new_ext, new_content = self.optimize_image(ext, content)
                    case FileType.VIDEO:
                        new_ext, new_content = self.optimize_video(ext, content)
                    case FileType.AUDIO:
                        new_ext, new_content = self.optimize_audio(ext, content)
                    case _:
                        raise ValueError(f"Unknown file type: {file_type}")

                new_filename = f'{hashlib.md5(new_content).hexdigest()}{new_ext}'
                self.write_file(file_type, new_filename, new_content)
                self.log_file_compress(file_type, filename, new_filename, len(content), len(new_content))
                return new_filename
            except Exception as e:
                self.write_file(file_type, filename, content)
                self.log_file_compress_error(file_type, filename, 0, str(e))
        else:
            self.log_file_compress_error(file_type, filename, 0, "File not found")
            return None

    def write_file(self, file_type: FileType, filename: str, content: bytes):
        match file_type:
            case FileType.IMAGE:
                self.dest.writestr(f'Images/{filename}', content)
            case FileType.VIDEO:
                self.dest.writestr(f'Video/{filename}', content)
            case FileType.AUDIO:
                self.dest.writestr(f'Audio/{filename}', content)
            case _:
                raise ValueError(f"Unknown file type: {file_type}")

    def log_file_compress(self, file_type: FileType, old_name: str, new_name: str, old_size: int, new_size: int):
        self.queue.put_nowait(LogEntrySchema(data=LogEntrySchema.LogEntryCompressed(
            type=file_type.value,
            old_name=old_name,
            new_name=new_name,
            old_size=old_size,
            new_size=new_size
        )))

    def log_file_compress_error(self, file_type: FileType, name: str, size: int, error: str):
        self.queue.put_nowait(LogEntrySchema(data=LogEntrySchema.LogEntryError(
            type=file_type.value,
            name=name,
            size=size,
            error=error
        )))
