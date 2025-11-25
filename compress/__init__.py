import os.path
from asyncio import Queue
from zipfile import ZipFile

import bs4

from compress.ops import v4, v5
from data import DONE_MARKER, ErrorSchema, LogEntrySchema, OptimizeResultSchema, PackInfoSchema


def compress(path: str, out_path: str, queue: Queue):
    with ZipFile(path, 'r') as source:
        with ZipFile(out_path, 'w') as dest:
            with source.open('content.xml', 'r') as cf:
                bs = bs4.BeautifulSoup(cf, 'lxml')

            version = int(bs.find('package').get('version'))
            if version not in [4, 5]:
                queue.put_nowait(ErrorSchema(error=f'Version {version} not supported'))
                queue.put_nowait(DONE_MARKER)
                return

            queue.put_nowait(PackInfoSchema(
                size=sum(file.file_size for file in source.infolist()),
                version=version,
                items_count=len(source.infolist()),
            ))

            ops = v4 if version == 4 else v5

            ops.optimize_files(source, dest, bs, queue)

            dest.writestr('content.xml', str(bs))
            queue.put_nowait(LogEntrySchema(content="Write content.xml..."))

            for file in ['Texts/sources.xml', 'Texts/authors.xml', 'quality.marker']:
                try:
                    dest.writestr(file, source.read(file))
                    queue.put_nowait(LogEntrySchema(content=f"Write {file}..."))
                except:
                    pass

    queue.put_nowait(OptimizeResultSchema(url=f'/download/{os.path.basename(out_path)}'))
    queue.put_nowait(DONE_MARKER)
