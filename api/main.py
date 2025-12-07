import hashlib
import logging
import os
import tempfile
import threading
from asyncio import Queue
from typing import Annotated

from fastapi import BackgroundTasks, FastAPI, File, HTTPException, Query, UploadFile
from pydantic import BaseModel
from starlette.middleware.cors import CORSMiddleware
from starlette.responses import FileResponse
from starlette.websockets import WebSocket

import compress
import db
from config import config
from data import DONE_MARKER

logger = logging.getLogger(__name__)

db.init()

app = FastAPI()

app.add_middleware(CORSMiddleware, allow_origins=["*"], allow_credentials=True, allow_methods=["*"], allow_headers=["*"])

queues = {}
lock = threading.Lock()


@app.post("/compress")
async def compress_pack(background_tasks: BackgroundTasks, file: UploadFile = File(), token: str = Query()):
    try:
        name, ext = os.path.splitext(file.filename)
        md5 = hashlib.md5()

        with tempfile.TemporaryFile('w+b') as temp_file:
            while chunk := await file.read(1024 * 1024):
                temp_file.write(chunk)
                md5.update(chunk)

            file_hash = md5.hexdigest()
            storage_file_path = config.storage_path / f'{file_hash}.siq'
            compressed_file_path = config.storage_path / f'{file_hash}-compressed.siq'

            temp_file.seek(0)
            with storage_file_path.open("wb") as storage_file:
                while chunk := temp_file.read(1024 * 1024):
                    storage_file.write(chunk)

        db.add_pack(file_hash, name)

        queue = init_queue(token)

        background_tasks.add_task(compress.compress, file_hash, storage_file_path, compressed_file_path, queue)

        return True
    except Exception as e:
        logger.exception(e)
        raise HTTPException(status_code=500, detail="Something went wrong")
    finally:
        file.file.close()


@app.get("/download/{hash}")
async def download(hash: str):
    if name := db.get_pack_name(hash):
        return FileResponse(config.storage_path / f'{hash}-compressed.siq', filename=f'{name}-compressed.siq')
    raise HTTPException(status_code=404, detail="Pack not found")


@app.websocket("/ws")
async def websocket_endpoint(*, websocket: WebSocket, token: Annotated[str | None, Query()] = None):
    await websocket.accept()
    queue = init_queue(token)
    while True:
        item = await queue.get()
        if item == DONE_MARKER:
            with lock:
                if token in queues:
                    del queues[token]
            await websocket.close()
            break
        await websocket.send_json(item.model_dump())


def init_queue(token: str) -> Queue[BaseModel]:
    with lock:
        if token not in queues:
            queues[token] = Queue()
        return queues[token]
