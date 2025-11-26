import math
import subprocess
from io import BytesIO

from PIL import Image


def optimize_image(ext: str, content: bytes) -> tuple[str, bytes]:
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
    cmd = ['ffmpeg', '-i', 'pipe:', '-f', 'mp4', '-movflags', 'isml+frag_keyframe', '-c:v', 'libx264', '-c:a', 'aac', '-vf', 'scale=trunc(oh*a/2)*2:664', '-crf', '28', '-loglevel', 'error', 'pipe:']
    proc = subprocess.Popen(cmd, stdout=subprocess.PIPE, stdin=subprocess.PIPE, stderr=subprocess.PIPE, bufsize=10 ** 8)
    out = proc.communicate(input=content)
    proc.wait()
    if not out[1]:
        return '.mp4', out[0]
    return ext, content


def optimize_audio(ext: str, content: bytes) -> tuple[str, bytes]:
    cmd = ['ffmpeg', '-i', 'pipe:', '-f', 'mp3', '-b:a', '96k', '-loglevel', 'error', 'pipe:']
    proc = subprocess.Popen(cmd, stdout=subprocess.PIPE, stdin=subprocess.PIPE, stderr=subprocess.PIPE, bufsize=10 ** 8)
    out = proc.communicate(input=content)
    proc.wait()
    if not out[1]:
        return '.mp3', out[0]
    return ext, content


def format_size(size: int) -> str:
    if size < 1024:
        return f'{size} B'
    if size < 1024 * 1024:
        return f'{size / 1024:.2f} KB'
    return f'{size / 1024 / 1024:.2f} MB'
