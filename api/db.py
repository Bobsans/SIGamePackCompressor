import sqlite3

from config import config

connection = sqlite3.connect(config.storage_path / "db.sipc")


def init():
    connection.execute("CREATE TABLE IF NOT EXISTS packs (hash UUID PRIMARY KEY, name TEXT)")


def add_pack(md5: str, name: str):
    cursor = connection.cursor()
    cursor.execute("INSERT INTO packs (hash, name) VALUES (?, ?) ON CONFLICT (hash) DO NOTHING", (md5, name))
    connection.commit()


def get_pack_name(md5: str) -> str | None:
    cursor = connection.cursor()
    cursor.execute("SELECT name FROM packs WHERE hash = ? LIMIT 1", (md5,))
    return row[0] if (row := cursor.fetchone()) else None
