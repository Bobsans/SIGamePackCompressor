import os.path
from asyncio import Queue
from zipfile import ZipFile

import bs4

from compress.ops import v4, v5


def compress(path: str, out_path: str, queue: Queue):
    with ZipFile(path, 'r') as source:
        with ZipFile(out_path, 'w') as dest:
            with source.open('content.xml', 'r') as cf:
                bs = bs4.BeautifulSoup(cf, 'lxml')

            version = int(bs.find('package').get('version'))
            if version not in [4, 5]:
                queue.put_nowait({'error': f'Version {version} not supported'})
                queue.put_nowait('DONE')
                return

            queue.put_nowait({
                'info': {
                    'size': sum(file.file_size for file in source.infolist()),
                    'items': len(source.infolist()),
                    'version': version
                }
            })

            ops = v4 if version == 4 else v5

            ops.optimize_files(source, dest, bs, queue)

            dest.writestr('content.xml', str(bs))
            queue.put_nowait("Write content.xml...")

            for file in ['Texts/sources.xml', 'Texts/authors.xml']:
                try:
                    dest.writestr(file, source.read(file))
                    queue.put_nowait(f"Write {file}...")
                except:
                    pass

    queue.put_nowait({'url': f'/dl/{os.path.basename(out_path)}'})
    queue.put_nowait('DONE')
