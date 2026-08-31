// IRC line parser — converts raw IRC protocol lines into structured objects
// for display in the live IRC panel. Strips IRC color/formatting codes and
// extracts nick, type, and content for IRC-client-style rendering.

export type IrcLineType =
  | "privmsg"    // channel message or private message
  | "notice"     // NOTICE from server or bot
  | "join"       // user joined channel
  | "quit"       // user quit IRC
  | "part"       // user left channel
  | "nick"       // user changed nick
  | "mode"       // mode change
  | "kick"       // user kicked
  | "topic"      // 332 — channel topic
  | "names"      // 353 — names list (collapsed)
  | "names_end"  // 366 — end of names
  | "welcome"    // 001 — registration confirmed
  | "motd"       // 372 — MOTD line
  | "motd_end"   // 376 — end of MOTD
  | "numeric"    // other server numerics (251, 252, etc.)
  | "dcc"        // DCC SEND/CHAT offer
  | "ping"       // PING from server
  | "error"      // ERROR from server
  | "raw";       // anything we couldn't parse

export interface ParsedIrcLine {
  type: IrcLineType;
  nick: string;       // sender nick (empty for server messages)
  hostmask: string;   // full hostmask (nick!user@host)
  target: string;     // target (#ebooks, our nick, etc.)
  content: string;    // message content (color codes stripped)
  raw: string;        // original raw line
  numeric: string;    // numeric code for server replies (e.g. "332", "001")
  timestamp: number;  // when the line was received
}

// IRC formatting control characters
const COLOR_CHAR = "\x03";
const BOLD_CHAR = "\x02";
const ITALIC_CHAR = "\x1d";
const UNDERLINE_CHAR = "\x1f";
const RESET_CHAR = "\x0f";
const REVERSE_CHAR = "\x16";

// Strip all IRC color and formatting codes, return plain text.
// Color code format: \x03[fg][,bg] where fg/bg are 1-2 digit numbers.
export function stripIrcColors(text: string): string {
  let out = "";
  for (let i = 0; i < text.length; i++) {
    const ch = text[i];
    if (ch === COLOR_CHAR) {
      // Skip color code: \x03 followed by optional 1-2 digit fg [, 1-2 digit bg]
      i++;
      // Skip foreground digits
      let digits = 0;
      while (i < text.length && /\d/.test(text[i]) && digits < 2) {
        i++;
        digits++;
      }
      // Skip comma + background digits
      if (i < text.length && text[i] === ",") {
        i++;
        digits = 0;
        while (i < text.length && /\d/.test(text[i]) && digits < 2) {
          i++;
          digits++;
        }
      }
      i--; // compensate for loop's i++
    } else if (
      ch === BOLD_CHAR ||
      ch === ITALIC_CHAR ||
      ch === UNDERLINE_CHAR ||
      ch === RESET_CHAR ||
      ch === REVERSE_CHAR
    ) {
      // Skip formatting characters
    } else {
      out += ch;
    }
  }
  return out.trim();
}

// Parse a raw IRC line into a structured object.
export function parseIrcLine(raw: string): ParsedIrcLine {
  const timestamp = Date.now();
  const line = raw;

  // PING :server
  if (line.startsWith("PING")) {
    return { type: "ping", nick: "", hostmask: "", target: "", content: line.slice(5), raw, numeric: "", timestamp };
  }

  // ERROR :message
  if (line.startsWith("ERROR")) {
    return { type: "error", nick: "", hostmask: "", target: "", content: stripIrcColors(line.slice(6)), raw, numeric: "", timestamp };
  }

  // Server numeric replies: :server NICK NUMERIC target :content
  // e.g. :zathras.mo.us.irchighway.net 332 openbooks_a_cde7 #ebooks :Topic text
  if (line.startsWith(":") && !line.startsWith(":") === false) {
    const spaceIdx = line.indexOf(" ");
    if (spaceIdx > 0) {
      const afterServer = line.slice(spaceIdx + 1);
      const parts = afterServer.split(" ");
      const numeric = parts[0];

      // Check if it's a 3-digit numeric reply
      if (/^\d{3}$/.test(numeric)) {
        const server = line.slice(1, spaceIdx);
        // Find the content after the last colon
        const colonIdx = line.indexOf(":", spaceIdx);
        const content = colonIdx >= 0 ? stripIrcColors(line.slice(colonIdx + 1)) : "";
        // Target is the part between numeric and content (usually our nick, maybe + channel)
        const targetParts = parts.slice(1, parts.length - 1).join(" ");

        const type = numericType(numeric);
        return {
          type,
          nick: server,
          hostmask: server,
          target: targetParts,
          content,
          raw,
          numeric,
          timestamp,
        };
      }

      // :nick!user@host COMMAND target :content
      const source = line.slice(1, spaceIdx);
      const afterSource = line.slice(spaceIdx + 1);
      const cmdParts = afterSource.split(" ");
      const command = cmdParts[0].toUpperCase();

      // Extract nick from source (nick!user@host or just server)
      const nickMatch = source.match(/^([^!]+)!/);
      const nick = nickMatch ? nickMatch[1] : source;

      // Find content after last colon
      const contentColonIdx = afterSource.indexOf(":");
      const content = contentColonIdx >= 0 ? stripIrcColors(afterSource.slice(contentColonIdx + 1)) : "";

      // Target is between command and content
      const target = cmdParts.slice(1).join(" ").split(":")[0].trim();

      switch (command) {
        case "PRIVMSG":
          // Check for DCC
          if (content.startsWith("DCC ")) {
            return { type: "dcc", nick, hostmask: source, target, content, raw, numeric: "", timestamp };
          }
          return { type: "privmsg", nick, hostmask: source, target, content, raw, numeric: "", timestamp };

        case "NOTICE":
          return { type: "notice", nick, hostmask: source, target, content, raw, numeric: "", timestamp };

        case "JOIN":
          return { type: "join", nick, hostmask: source, target: content || target, content: "", raw, numeric: "", timestamp };

        case "QUIT":
          return { type: "quit", nick, hostmask: source, target: "", content, raw, numeric: "", timestamp };

        case "PART":
          return { type: "part", nick, hostmask: source, target, content, raw, numeric: "", timestamp };

        case "NICK":
          return { type: "nick", nick, hostmask: source, target: "", content: content || target, raw, numeric: "", timestamp };

        case "MODE":
          return { type: "mode", nick, hostmask: source, target, content: afterSource.slice(command.length + 1), raw, numeric: "", timestamp };

        case "KICK":
          return { type: "kick", nick, hostmask: source, target, content, raw, numeric: "", timestamp };

        default:
          return { type: "raw", nick, hostmask: source, target, content: afterSource, raw, numeric: "", timestamp };
      }
    }
  }

  return { type: "raw", nick: "", hostmask: "", target: "", content: stripIrcColors(line), raw, numeric: "", timestamp };
}

function numericType(code: string): IrcLineType {
  switch (code) {
    case "001": return "welcome";
    case "332": return "topic";
    case "353": return "names";
    case "366": return "names_end";
    case "372": return "motd";
    case "375": return "motd"; // MOTD start
    case "376": return "motd_end";
    default: return "numeric";
  }
}

// Generate a consistent color from a nick for IRC-client-style nick coloring.
// Returns a hue (0-360) that's stable for the same nick.
const nickHueCache = new Map<string, number>();

export function nickHue(nick: string): number {
  if (!nick) return 0;
  const cached = nickHueCache.get(nick);
  if (cached !== undefined) return cached;

  let hash = 0;
  for (let i = 0; i < nick.length; i++) {
    hash = (hash * 31 + nick.charCodeAt(i)) | 0;
  }
  const hue = Math.abs(hash) % 360;
  nickHueCache.set(nick, hue);
  return hue;
}

// Format a timestamp as HH:MM:SS for display.
export function formatIrcTime(ts: number): string {
  const d = new Date(ts);
  return d.toLocaleTimeString("en-US", { hour12: false });
}
