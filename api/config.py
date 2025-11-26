import logging
from pathlib import Path

import yaml
from pydantic import BaseModel, field_validator

BASE_PATH = Path(__file__).parent


class ConfigSchema(BaseModel):
    storage_path: Path = BASE_PATH / "storage"

    @field_validator("storage_path")
    def validate_storage_path(cls, v):
        return Path(v).resolve()


config = ConfigSchema()

try:
    with open(BASE_PATH / "config.yaml", "r") as file:
        config = ConfigSchema.model_validate(yaml.safe_load(file))
except (FileNotFoundError, yaml.YAMLError) as e:
    logging.getLogger(__name__).error(f"Config not loaded: {e}")

if not config.storage_path.exists():
    config.storage_path.mkdir(exist_ok=True)
