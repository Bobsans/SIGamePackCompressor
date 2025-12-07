def format_size(size: int) -> str:
    if size < 1024:
        return f'{size} B'
    if size < 1024 * 1024:
        return f'{size / 1024:.2f} KB'
    return f'{size / 1024 / 1024:.2f} MB'
