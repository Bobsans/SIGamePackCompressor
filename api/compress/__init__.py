from asyncio import Queue
from pathlib import Path
from zipfile import ZIP_DEFLATED, ZipFile

import bs4

from compress.ops import v4, v5
from data import DONE_MARKER, ErrorSchema, LogEntrySchema, OptimizeResultSchema, PackInfoSchema


def compress(path: Path, out_path: Path, queue: Queue):
    file_size = path.stat().st_size

    with ZipFile(path, 'r') as source:
        with ZipFile(out_path, 'w', ZIP_DEFLATED, compresslevel=9) as dest:
            with source.open('content.xml', 'r') as cf:
                bs = bs4.BeautifulSoup(cf, 'xml')

            version = int(bs.find('package').get('version'))
            ops = {4: v4, 5: v5}.get(version, None)

            if ops is None:
                queue.put_nowait(ErrorSchema(error=f'Pack version {version} not supported'))
                queue.put_nowait(DONE_MARKER)
                return

            queue.put_nowait(PackInfoSchema(
                size=file_size,
                version=version,
                items_count=ops.get_optimizable_items_count(source),
            ))

            ops.optimize_files(source, dest, bs, queue)

            dest.writestr('content.xml', str(bs))
            queue.put_nowait(LogEntrySchema(content="Write content.xml..."))

            for file in ['Texts/sources.xml', 'Texts/authors.xml', 'quality.marker']:
                try:
                    dest.writestr(file, source.read(file))
                    queue.put_nowait(LogEntrySchema(content=f"Write {file}..."))
                except:
                    pass

    queue.put_nowait(OptimizeResultSchema(url=f'/download/{out_path.name}'))
    queue.put_nowait(DONE_MARKER)
