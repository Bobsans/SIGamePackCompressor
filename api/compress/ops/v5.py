import urllib.parse

from compress.ops import CompressorBase, FileType


class Compressor(CompressorBase):
    def get_optimizable_items_count(self) -> int:
        return len([file for file in self.source.infolist() if file.filename.startswith('Images/') or file.filename.startswith('Video/') or file.filename.startswith('Audio/')])

    def compress(self):
        package = self.soup.find('package')

        if logo := package.get('logo'):
            filename = logo.replace('@', '')
            if new_filename := self.optimize_file(FileType.IMAGE, filename):
                package.attrs['logo'] = f'@{new_filename}'

        images = self.soup.find_all('item', attrs={'type': 'image'})
        videos = self.soup.find_all('item', attrs={'type': 'video'})
        audios = self.soup.find_all('item', attrs={'type': 'audio'})

        for tag in [*images, *videos, *audios]:
            filename = tag.text
            tile_type = FileType(tag.get('type'))

            if new_filename := self.optimize_file(tile_type, filename):
                tag.string.replace_with(f'{new_filename}')

    def get_file_content(self, file_type: FileType, filename: str) -> bytes | None:
        path = filename.replace('@', '')
        paths = [
            urllib.parse.quote(path.replace('&amp;', '&'), safe='!@#$&()[]{}+-=_;\'.,').replace(' ', '%20').strip(),
            urllib.parse.quote(path.replace('&amp;', '&'), safe='!@#$&()[]{}+-=_;\'.,').strip(),
            path.strip()
        ]

        for path in paths:
            match file_type:
                case FileType.IMAGE:
                    path = f'Images/{path}'
                case FileType.VIDEO:
                    path = f'Video/{path}'
                case FileType.AUDIO:
                    path = f'Audio/{path}'
                case _:
                    raise ValueError(f"Unknown file type: {file_type}")

            if self.source.getinfo(path):
                with self.source.open(path, 'r') as file:
                    return file.read()

        return None
