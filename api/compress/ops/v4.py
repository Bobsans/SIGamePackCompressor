import hashlib
import os
import urllib.parse
from asyncio import Queue
from zipfile import ZipFile

from bs4 import BeautifulSoup

from compress.ops import optimize_audio, optimize_image, optimize_video
from data import LogEntrySchema
from utils import format_size


def get_optimizable_items_count(source: ZipFile) -> int:
    return len([file for file in source.infolist() if file.filename.startswith('Images/') or file.filename.startswith('Video/') or file.filename.startswith('Audio/')])


def optimize_files(source: ZipFile, dest: ZipFile, bs: BeautifulSoup, queue: Queue):
    package = bs.find('package')

    if logo := package.get('logo'):
        try:
            name, ext = os.path.splitext(logo.replace('@', ''))
            original_content = get_file_content(source, logo, 'image')
            ext, content = optimize_image(ext, original_content)
            name = hashlib.md5(content).hexdigest()
            dest.writestr(f'Images/{name}{ext}', content)
            package.attrs['logo'] = f'@{name}{ext}'
            queue.put_nowait(LogEntrySchema(content=f'Logo optimized! [{format_size(len(original_content))} -> {format_size(len(content))}]'))
        except Exception as e:
            queue.put_nowait(LogEntrySchema(content=f'Logo optimization error! {e}'))

    images = bs.find_all('atom', attrs={'type': 'image'})
    videos = bs.find_all('atom', attrs={'type': 'video'})
    audios = bs.find_all('atom', attrs={'type': 'voice'})

    for tag in [*images, *videos, *audios]:
        type = tag.get('type')
        filename = tag.text
        name, ext = os.path.splitext(filename.replace('@', ''))

        if type == 'image':
            try:
                original_content = get_file_content(source, filename, type)
                ext, content = optimize_image(ext, original_content)
                hashname = hashlib.md5(content).hexdigest()
                dest.writestr(f'Images/{hashname}{ext}', content)
                tag.string.replace_with(f'@{hashname}{ext}')
                queue.put_nowait(LogEntrySchema(content=f'Image {name}{ext} [{format_size(len(original_content))}] -> {hashname}{ext} [{format_size(len(content))}]'))
            except Exception as e:
                queue.put_nowait(LogEntrySchema(content=f'Image optimization error! {e}'))
        elif type == 'video':
            try:
                original_content = get_file_content(source, filename, type)
                ext, content = optimize_video(ext, original_content)
                hashname = hashlib.md5(content).hexdigest()
                dest.writestr(f'Video/{hashname}{ext}', content)
                tag.string.replace_with(f'@{hashname}{ext}')
                queue.put_nowait(LogEntrySchema(content=f'Video {name}{ext} [{format_size(len(original_content))}] -> {hashname}{ext} [{format_size(len(content))}]'))
            except Exception as e:
                queue.put_nowait(LogEntrySchema(content=f'Video optimization error! {e}'))
        elif type == 'voice':
            try:
                original_content = get_file_content(source, filename, type)
                ext, content = optimize_audio(ext, original_content)
                hashname = hashlib.md5(content).hexdigest()
                dest.writestr(f'Audio/{hashname}{ext}', content)
                tag.string.replace_with(f'@{hashname}{ext}')
                queue.put_nowait(LogEntrySchema(content=f'Audio {name}{ext} [{format_size(len(original_content))}] -> {hashname}{ext} [{format_size(len(content))}]'))
            except Exception as e:
                queue.put_nowait(LogEntrySchema(content=f'Audio optimization error! {e}'))


def get_file_content(source: ZipFile, path: str, type: str) -> bytes:
    path = path.replace('@', '')
    path = urllib.parse.quote(path.replace('&amp;', '&'), safe='!@#$&()[]{}+-=_;\'.,').replace(' ', '%20')

    if type == 'image':
        path = f'Images/{path}'
    elif type == 'video':
        path = f'Video/{path}'
    elif type == 'voice':
        path = f'Audio/{path}'

    with source.open(path, 'r') as file:
        return file.read()
