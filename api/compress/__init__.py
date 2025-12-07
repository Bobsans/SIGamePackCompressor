import logging
import os
from asyncio import Queue
from pathlib import Path
from zipfile import ZIP_DEFLATED, ZipFile

import bs4

from compress.ops import v4, v5
from data import DONE_MARKER, LogEntrySchema, OptimizeResultSchema, PackInfoSchema

logger = logging.getLogger(__name__)


def compress(file_hash: str, path: Path, out_path: Path, queue: Queue):
    file_size = path.stat().st_size
    content_file_name = 'content.xml'

    with ZipFile(path, 'r') as source:
        with ZipFile(out_path, 'w', ZIP_DEFLATED, compresslevel=9) as dest:
            with source.open(content_file_name, 'r') as cf:
                soup = bs4.BeautifulSoup(cf, 'xml')

            version = int(soup.find('package').get('version'))
            ops = {4: v4, 5: v5}.get(version, None)

            if ops is None:
                queue.put_nowait(LogEntrySchema(data=LogEntrySchema.LogEntryError(error=f'Pack version {version} not supported')))
                queue.put_nowait(DONE_MARKER)
                return

            compressor = ops.Compressor(source, dest, soup, queue)

            queue.put_nowait(PackInfoSchema(
                size=file_size,
                version=version,
                items_count=compressor.get_optimizable_items_count(),
            ))

            compressor.compress()

            content = str(soup)
            dest.writestr(content_file_name, content)
            queue.put_nowait(LogEntrySchema(data=LogEntrySchema.LogEntryCompressed(
                type="xml",
                old_name=content_file_name,
                new_name=content_file_name,
                old_size=source.getinfo(content_file_name).file_size,
                new_size=len(content)
            )))

            for file in ['Texts/sources.xml', 'Texts/authors.xml', 'quality.marker']:
                if (info := source.NameToInfo.get(file, None)) is not None:
                    dest.writestr(file, source.read(file))
                    queue.put_nowait(LogEntrySchema(data=LogEntrySchema.LogEntryCompressed(
                        type="xml",
                        old_name=os.path.basename(file),
                        new_name=os.path.basename(file),
                        old_size=info.file_size,
                        new_size=info.file_size
                    )))

    queue.put_nowait(OptimizeResultSchema(url=f'/download/{file_hash}'))
    queue.put_nowait(DONE_MARKER)
