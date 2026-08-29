const VNID_IN_TEXT = /(?:^|\/)(v\d+)(?:\/?(?:[?#].*)?$)/i;

export function parseVndbId(input: string): string | null {
  const trimmed = input.trim();
  if (!trimmed) {
    return null;
  }
  if (/^v\d+$/i.test(trimmed)) {
    return trimmed.toLowerCase();
  }
  const match = trimmed.match(VNID_IN_TEXT);
  return match ? match[1].toLowerCase() : null;
}
