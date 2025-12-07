import logging
import math
import subprocess
from enum import Enum
from io import BytesIO

from PIL import Image

logger = logging.getLogger(__name__)


class FileType(Enum):
    IMAGE = 'image'
    VIDEO = 'video'
    AUDIO = 'audio'


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
