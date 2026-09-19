const MAX_TAG_LENGTH = 20;

export function extractTagTemplateParams(content: string): {
  params: string[];
  error?: string;
} {
  const params: string[] = [];
  const seen = new Set<string>();
  const chars = Array.from(content);
  for (let i = 0; i < chars.length; ) {
    const ch = chars[i];
    if (ch === "}") {
      return { params: [], error: "invalid template: unmatched '}'" };
    }
    if (ch !== "{") {
      i++;
      continue;
    }
    i++;
    const start = i;
    while (i < chars.length && chars[i] !== "}") {
      if (chars[i] === "{") {
        return { params: [], error: "invalid template: nested '{'" };
      }
      i++;
    }
    if (i >= chars.length) {
      return { params: [], error: "invalid template: unmatched '{'" };
    }
    const name = chars.slice(start, i).join("");
    if (!isValidTemplateParamName(name)) {
      return { params: [], error: "invalid template: invalid parameter name" };
    }
    if (!seen.has(name)) {
      seen.add(name);
      params.push(name);
    }
    i++;
  }
  return { params };
}

function isValidTemplateParamName(name: string): boolean {
  if (!name) {
    return false;
  }
  for (const ch of name) {
    if (ch === "," || /\s/.test(ch)) {
      return false;
    }
  }
  return true;
}

export function previewTagTemplate(
  content: string,
  values: Record<string, string>,
): { tags: string[]; error?: string } {
  const parsed = extractTagTemplateParams(content);
  if (parsed.error) {
    return { tags: [], error: parsed.error };
  }
  const chars = Array.from(content);
  const out: string[] = [];
  for (let i = 0; i < chars.length; ) {
    if (chars[i] !== "{") {
      out.push(chars[i]);
      i++;
      continue;
    }
    i++;
    const start = i;
    while (i < chars.length && chars[i] !== "}") {
      i++;
    }
    const name = chars.slice(start, i).join("");
    const raw = values[name];
    if (raw === undefined) {
      out.push("{", ...Array.from(name), "}");
      i++;
      continue;
    }
    const value = raw.trim();
    if (!value) {
      return { tags: [], error: `parameter cannot be empty: ${name}` };
    }
    if (/[{},]/.test(value)) {
      return {
        tags: [],
        error: `parameter value cannot contain braces or commas: ${name}`,
      };
    }
    out.push(...Array.from(value));
    i++;
  }
  const tags: string[] = [];
  const seen = new Set<string>();
  for (const part of out.join("").split(",")) {
    const name = part.trim();
    if (!name || seen.has(name)) {
      continue;
    }
    if (!name.includes("{") && Array.from(name).length > MAX_TAG_LENGTH) {
      return { tags: [], error: `Tag name too long: ${name}` };
    }
    seen.add(name);
    tags.push(name);
  }
  if (tags.length === 0) {
    return { tags: [], error: "template produces no tags" };
  }
  return { tags };
}
