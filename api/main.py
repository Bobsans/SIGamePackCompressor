import logging
import os
import threading
from asyncio import Queue
from typing import Annotated

from fastapi import BackgroundTasks, FastAPI, File, HTTPException, Query, UploadFile
from pydantic import BaseModel
from starlette.middleware.cors import CORSMiddleware
from starlette.websockets import WebSocket

import compress
from config import config
from data import DONE_MARKER

logger = logging.getLogger(__name__)
app = FastAPI()

app.add_middleware(CORSMiddleware, allow_origins=["*"], allow_credentials=True, allow_methods=["*"], allow_headers=["*"])

queues = {}
lock = threading.Lock()


@app.post("/compress")
async def compress_pack(background_tasks: BackgroundTasks, file: UploadFile = File(), token: str = Query()):
    try:
        name, ext = os.path.splitext(file.filename)
        file_path = config.storage_path / f'{name}.siq'
        out_file_name = f'{name}-compressed.siq'
        out_file_path = config.storage_path / out_file_name

        with file_path.open("wb") as f:
            while chunk := await file.read(1024 * 1024):
                f.write(chunk)

        queue = init_queue(token)

        background_tasks.add_task(compress.compress, file_path, out_file_path, queue)

        return True
    except Exception as e:
        logger.exception(e)
        raise HTTPException(status_code=500, detail="Something went wrong")
    finally:
        file.file.close()


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
