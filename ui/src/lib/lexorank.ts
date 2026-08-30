const BASE = 36;
const DIGITS = "0123456789abcdefghijklmnopqrstuvwxyz";

function digitVal(c: string): number {
  if (c >= "0" && c <= "9") return c.charCodeAt(0) - "0".charCodeAt(0);
  if (c >= "a" && c <= "z") return c.charCodeAt(0) - "a".charCodeAt(0) + 10;
  return -1;
}

function valChar(v: number): string {
  if (v < 0 || v >= BASE) return "0";
  return DIGITS[v];
}

function validate(s: string): void {
  for (let i = 0; i < s.length; i++) {
    if (digitVal(s[i]) < 0) {
      throw new Error("invalid character in key: " + s[i]);
    }
  }
}

function before(b: string): string {
  for (let i = 0; i < b.length; i++) {
    const db = digitVal(b[i]);
    if (db > 0) {
      return b.slice(0, i) + valChar(Math.floor(db / 2));
    }
  }
  throw new Error("no room before " + b);
}

function after(a: string): string {
  for (let i = 0; i < a.length; i++) {
    const da = digitVal(a[i]);
    if (da < BASE - 1) {
      const avg = Math.floor((da + BASE) / 2);
      return a.slice(0, i) + valChar(avg);
    }
  }
  return a + "z";
}

export function generateKeyBetween(
  a: string | null,
  b: string | null,
): string {
  if (a !== null) validate(a);
  if (b !== null) validate(b);

  if (a === null && b === null) {
    return "h";
  }
  if (a === null) {
    return before(b!);
  }
  if (b === null) {
    return after(a);
  }

  if (a >= b) {
    if (a === b) {
      throw new Error("keys are identical");
    }
    throw new Error("a must be less than b: " + a + " >= " + b);
  }

  let i = 0;
  while (i < a.length && i < b.length && a[i] === b[i]) {
    i++;
  }

  const prefix = a.slice(0, i);

  if (i >= a.length) {
    const db = digitVal(b[i]);
    const avg = Math.floor(db / 2);
    if (avg < db) {
      return prefix + valChar(avg);
    }
    throw new Error("no room between " + a + " and " + b);
  }

  if (i >= b.length) {
    throw new Error("no room between " + a + " and " + b);
  }

  const da = digitVal(a[i]);
  const db = digitVal(b[i]);

  if (da >= db) {
    throw new Error("invalid ordering: " + a + " vs " + b);
  }

  const avg = Math.floor((da + db) / 2);

  if (avg > da) {
    return prefix + valChar(avg);
  }

  let result = prefix + a[i];

  for (let j = i + 1; j < a.length; j++) {
    const dj = digitVal(a[j]);
    if (dj < BASE - 1) {
      return result + a.slice(i + 1, j) + valChar(dj + 1);
    }
    result += a[j];
  }

  return result + "5";
}

export function getFirstKey(): string {
  return "h";
}

export function isValidKey(key: string): boolean {
  if (key.length === 0) return false;
  for (let i = 0; i < key.length; i++) {
    if (digitVal(key[i]) < 0) return false;
  }
  return true;
}
