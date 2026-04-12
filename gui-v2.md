# OpenBooks GUI v2 — Vue 3 Rewrite Plan

## Is This Difficult?

**Short answer: moderate effort, not extreme difficulty.**

The backend (Go server, WebSocket protocol, REST API) stays entirely unchanged. All we're replacing is `server/app/` — the frontend bundle. The current feature surface is well-scoped: one page, a sidebar, a table, and WebSocket plumbing. A solo developer with AI assistance can realistically complete this in 3–5 focused sessions.

The main cost is translating Redux + React idioms into Vue/Pinia idioms and rebuilding the responsive layout from scratch with a cleaner design system.

---

## Current Stack (what we're replacing)

| Concern | Current |
|---|---|
| Framework | React 18 + TypeScript |
| UI Library | Mantine v5 |
| State | Redux Toolkit |
| Table | TanStack Table v8 |
| Virtualization | TanStack Virtual v3 |
| Icons | Phosphor React |
| Build | Vite |

**Pain points of the current design:**
- Mantine v5 is now quite dated (v7 is a full rewrite, hard to upgrade)
- Redux boilerplate is heavy for a single-page app
- Mobile sidebar experience (bottom drawer + FAB) is awkward
- Theme customisation is locked inside Mantine's emotion cache
- No proper route-level separation (everything in one `SearchPage`)

---

## Proposed Stack (Vue 3)

| Concern | Chosen |
|---|---|
| Framework | Vue 3 + TypeScript + `<script setup>` |
| Build | Vite (same as now) |
| Styling | TailwindCSS v3 |
| Components | shadcn-vue (Radix-based, headless, accessible) |
| State | Pinia |
| Table | @tanstack/vue-table v8 |
| Virtualization | @tanstack/vue-virtual v3 |
| Icons | Lucide Vue Next |
| HTTP | @tanstack/vue-query (replaces RTK Query) |
| Composables | VueUse |
| Notifications | vue-sonner (shadcn-style toasts) |

**Why this stack?**
- Pinia is the official Vue state library — far less boilerplate than Redux
- TailwindCSS gives complete design freedom without a component library lock-in
- shadcn-vue components are copy-paste into `src/components/ui/`, fully customisable
- TanStack Table/Virtual have first-class Vue adapters
- VueUse provides `useMediaQuery`, `useLocalStorage`, `useDark`, etc. — replaces all Mantine hooks

---

## File Structure

```
server/app/          ← replace contents (keep the directory, Go embeds it)
├── index.html
├── package.json
├── vite.config.ts
├── tailwind.config.ts
├── tsconfig.json
└── src/
    ├── main.ts
    ├── App.vue
    ├── router/
    │   └── index.ts
    ├── stores/
    │   ├── app.ts          ← connection state, username, activeItem, sidebarOpen
    │   ├── history.ts      ← search history (persisted)
    │   └── notifications.ts← toast queue
    ├── composables/
    │   ├── useWebSocket.ts ← WS connect/reconnect, message routing
    │   └── useDownload.ts  ← in-flight download tracking
    ├── types/
    │   └── messages.ts     ← BookDetail, ParseError, MessageType, etc. (unchanged)
    ├── components/
    │   ├── ui/             ← shadcn-vue primitives (Button, Input, Badge, etc.)
    │   ├── layout/
    │   │   ├── AppShell.vue        ← desktop: sidebar + main, mobile: bottom nav
    │   │   ├── Sidebar.vue         ← collapsible left panel
    │   │   ├── MobileNav.vue       ← bottom nav bar on mobile
    │   │   └── ThemeToggle.vue
    │   ├── search/
    │   │   ├── SearchBar.vue
    │   │   └── EmptyState.vue
    │   ├── books/
    │   │   ├── BookTable.vue       ← desktop virtualised table
    │   │   ├── BookCards.vue       ← mobile card list
    │   │   ├── DownloadButton.vue
    │   │   └── FilterDrawer.vue    ← mobile filter sheet
    │   ├── sidebar/
    │   │   ├── HistoryPanel.vue
    │   │   ├── LibraryPanel.vue
    │   │   └── LogsPanel.vue
    │   └── errors/
    │       └── ErrorTable.vue
    └── pages/
        └── SearchView.vue          ← main/only page
```

---

## Layout Design

### Desktop (≥ 768 px)

```
┌─────────────────────────────────────────────────────┐
│  ┌──────────────┐  ┌──────────────────────────────┐ │
│  │   SIDEBAR    │  │  [≡] [Search input........] [Go] │
│  │              │  │                              │ │
│  │  ○ History   │  │  ┌──────────────────────┐   │ │
│  │  ○ Downloads │  │  │  Book Results Table  │   │ │
│  │  ○ Logs      │  │  │  (virtualised)       │   │ │
│  │              │  │  └──────────────────────┘   │ │
│  │  ───────     │  │                              │ │
│  │  ● username  │  │                              │ │
│  │  v1.2.3 ☀ ◧ │  └──────────────────────────────┘ │
│  └──────────────┘                                   │
└─────────────────────────────────────────────────────┘
```

- Sidebar is **collapsible** — toggles with a button, persists in localStorage
- Sidebar width: 280 px
- When collapsed, only the main panel shows; a hamburger icon re-opens it
- Dark/light toggle in sidebar footer

### Mobile (< 768 px)

```
┌──────────────────────┐
│  OpenBooks        ☰  │  ← top bar with title + menu icon
├──────────────────────┤
│                      │
│  [Search........] [Go]│
│                      │
│  ┌──────────────┐   │
│  │  Book card   │   │
│  └──────────────┘   │
│  ┌──────────────┐   │
│  │  Book card   │   │
│  └──────────────┘   │
│         ...          │
├──────────────────────┤
│  🔍History │📚Books│📋Logs │  ← bottom navigation bar
└──────────────────────┘
```

- **Bottom tab bar** replaces the current "pull-up drawer + FAB" pattern
- Sheet/drawer slides up from bottom when a tab is tapped (History, Downloads, Logs)
- No floating action button needed
- Safe-area insets handled via Tailwind `pb-safe`

---

## State Management (Pinia)

### `stores/app.ts`
```ts
// Replaces stateSlice.ts
interface AppState {
  isConnected: boolean
  isSidebarOpen: boolean
  activeItem: HistoryItem | null
  username: string | undefined
  inFlightDownloads: string[]
}
```
Actions: `sendSearch()`, `sendDownload()`, `sendMessage()`, `setSearchResults()`

### `stores/history.ts`
```ts
// Replaces historySlice.ts
// Persisted to localStorage via pinia-plugin-persistedstate
interface HistoryState {
  items: HistoryItem[]
}
```

### `stores/notifications.ts`
```ts
// Replaces notificationSlice.ts
// Drives vue-sonner toast calls directly (no persistent queue needed)
```

---

## WebSocket Layer

The WS reconnection logic in `socketMiddleware.ts` becomes a **composable** `useWebSocket.ts` that is called once in `App.vue`. It holds the same exponential backoff logic and queues outbound messages while disconnected.

```ts
// composables/useWebSocket.ts
export function useWebSocket(url: string) {
  // same MAX_RETRIES / backoff logic
  // emits routed payloads via pinia store actions
  // exposes: send(message), isConnected (ref)
}
```

The message routing (`route()`) moves into the composable and calls pinia store actions directly instead of Redux dispatch.

---

## Component Mapping (Old → New)

| Old (React) | New (Vue) |
|---|---|
| `App.tsx` | `App.vue` + `AppShell.vue` |
| `Sidebar.tsx` | `Sidebar.vue` |
| `ActivityLog.tsx` | `LogsPanel.vue` |
| `History.tsx` | `HistoryPanel.vue` |
| `Library.tsx` | `LibraryPanel.vue` |
| `SearchPage.tsx` | `SearchView.vue` + `SearchBar.vue` |
| `BookTable.tsx` | `BookTable.vue` (desktop) + `BookCards.vue` (mobile) |
| `ErrorTable.tsx` | `ErrorTable.vue` |
| `NotificationDrawer.tsx` | `vue-sonner` toasts (no persistent drawer) |
| Redux middleware | `useWebSocket.ts` composable |
| RTK Query | `@tanstack/vue-query` |

---

## REST API (unchanged)

The Go server exposes the same endpoints — only the frontend changes.

| Endpoint | Purpose |
|---|---|
| `GET /ws` | WebSocket connection |
| `GET /servers` | Online IRC servers list |
| `GET /version` | App version string |
| `GET /logs` | Server log entries |
| `GET /library` | Downloaded books |
| `DELETE /library/*` | Delete a book |
| `GET /library/*` | Download a book file |

---

## WebSocket Message Types (unchanged)

```ts
enum MessageType { STATUS, CONNECT, SEARCH, DOWNLOAD, RATELIMIT }
```

Message shapes stay identical — the Go backend doesn't change at all.

---

## Visual Design Decisions

- **Color scheme**: Neutral gray base with a blue accent (close to current brand blue `#3366ff`)
- **Dark mode**: System-preference default via `useDark()` from VueUse, manually overridable
- **Typography**: System font stack (no web font download overhead)
- **Border radius**: `rounded-lg` throughout for modern card feel
- **Shadows**: Subtle — `shadow-sm` on cards, `shadow-md` on drawers
- **Transitions**: `transition-all duration-200` on sidebar open/close

---

## Implementation Phases

### Phase 1 — Scaffold & WebSocket
1. Replace `server/app/` with a fresh `npm create vue@latest` (Vue 3 + TS + Vite)
2. Install dependencies (Tailwind, Pinia, shadcn-vue, Lucide, VueUse)
3. Port `types/messages.ts`
4. Implement `useWebSocket.ts` composable with reconnect
5. Wire up Pinia stores (app, history, notifications)
6. Verify WS connect/search/download round-trip works

### Phase 2 — Desktop Layout
7. Build `AppShell.vue` with collapsible sidebar
8. Build `SearchBar.vue`
9. Build `BookTable.vue` with TanStack Table + Virtual
10. Build `ErrorTable.vue`
11. Build `EmptyState.vue`

### Phase 3 — Sidebar Panels
12. `HistoryPanel.vue` — search history with click-to-restore
13. `LibraryPanel.vue` — downloaded books with delete + re-download
14. `LogsPanel.vue` — scrollable log entries

### Phase 4 — Mobile Layout
15. Build `MobileNav.vue` (bottom tab bar)
16. Build `BookCards.vue` (virtualised card list)
17. Build `FilterDrawer.vue` (bottom sheet for mobile filters)
18. Verify responsive breakpoints on small screens

### Phase 5 — Polish
19. Notifications via `vue-sonner`
20. Dark mode toggle
21. Connection status indicator
22. Version display
23. Transition animations

---

## Build Integration

The Go embed directive points to `server/app/dist`:

```go
//go:embed app/dist
var reactClient embed.FS
```

The new Vue app builds to the same `server/app/dist/` directory, so no Go changes are needed. The Vite config base path must match the existing `Basepath` server config.

---

## Dependencies (new `package.json`)

```json
{
  "dependencies": {
    "vue": "^3.4",
    "pinia": "^2.1",
    "pinia-plugin-persistedstate": "^3.2",
    "@tanstack/vue-table": "^8.17",
    "@tanstack/vue-virtual": "^3.8",
    "@tanstack/vue-query": "^5.40",
    "@vueuse/core": "^10.9",
    "lucide-vue-next": "^0.378",
    "vue-sonner": "^1.1",
    "class-variance-authority": "^0.7",
    "clsx": "^2.1",
    "tailwind-merge": "^2.3",
    "radix-vue": "^1.9"
  },
  "devDependencies": {
    "@vitejs/plugin-vue": "^5.0",
    "typescript": "^5.4",
    "vite": "^6.3",
    "tailwindcss": "^3.4",
    "autoprefixer": "^10.4",
    "postcss": "^8.4",
    "vue-tsc": "^2.0"
  }
}
```

---

## Open Questions / Decisions Needed

1. **Keep the existing `server/app/` directory** (recommended — Go embed stays happy) or create `server/app-vue/` and update the embed path?
2. **shadcn-vue vs PrimeVue**: shadcn-vue gives more design control; PrimeVue has more out-of-the-box components. Recommend shadcn-vue for this scale.
3. **Notification drawer**: The current app has a persistent notification history drawer. Should v2 keep that, or use ephemeral toasts only (simpler)?
4. **Router**: The app currently has no routes. Should v2 add routes (e.g., `/library`, `/logs` as dedicated pages) or stay single-page?
