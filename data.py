from pydantic import BaseModel

DONE_MARKER = object()


class PackInfoSchema(BaseModel):
    type: str = "info"

    size: int
    version: int
    items_count: int


class LogEntrySchema(BaseModel):
    type: str = "log"

    content: str


class ErrorSchema(BaseModel):
    type: str = "error"

    error: str


class OptimizeResultSchema(BaseModel):
    type: str = "result"

    url: str
