from pydantic import BaseModel

DONE_MARKER = 0xFFFFFF


class PackInfoSchema(BaseModel):
    type: str = "info"

    size: int
    version: int
    items_count: int


class OptimizeResultSchema(BaseModel):
    type: str = "result"

    url: str


class LogEntrySchema(BaseModel):
    type: str = "log"

    data: LogEntryCompressed | LogEntryError

    class LogEntryCompressed(BaseModel):
        event: str = "compressed"

        type: str
        old_name: str
        new_name: str
        old_size: int
        new_size: int

    class LogEntryError(BaseModel):
        event: str = "error"

        type: str | None = None
        name: str | None = None
        size: int | None = None
        error: str
