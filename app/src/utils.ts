export const urljoin = (...args: string[]) => args.map(it => it.replace(/^\/|\/$/g, "")).join("/");

export const formatSize = (size: number) => {
  const value = Math.max(0, Number.isFinite(size) ? size : 0);
  const unit = value >= 1024 ** 3 ? 3 : value >= 1024 ** 2 ? 2 : value >= 1024 ? 1 : 0;
  const formatted = new Intl.NumberFormat("ru-RU", { maximumFractionDigits: unit ? 2 : 0 }).format(value / 1024 ** unit);
  return `${formatted} ${["Б", "КБ", "МБ", "ГБ"][unit]}`;
};

export function fileCount(count: number) {
  const last = count % 10;
  const teen = count % 100 >= 11 && count % 100 <= 14;
  const word = !teen && last === 1 ? "файл" : !teen && last >= 2 && last <= 4 ? "файла" : "файлов";
  return `${count} ${word}`;
}

