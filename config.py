import logging
import os

import yaml
from pydantic import BaseModel


class ConfigSchema(BaseModel):
    base_path: str = os.path.dirname(__file__)
    storage_path: str = os.path.join("storage")


config = ConfigSchema()

try:
    with open("config.yaml", "r") as file:
        config = ConfigSchema.model_validate(yaml.safe_load(file))
except (FileNotFoundError, yaml.YAMLError) as e:
    logging.getLogger(__name__).error(f"Config not loaded: {e}")
