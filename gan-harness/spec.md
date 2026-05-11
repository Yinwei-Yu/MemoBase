# Frontend UI Redesign — KnowledgeAI Console

## Current State
- React 18 + TypeScript + Vite, no CSS framework
- Pure CSS with glassmorphism aesthetic (frosted glass, blue gradients, Inter font)
- Zustand + React Query + react-router-dom
- 6 pages: Login, KnowledgeBase, Documents, Chat, Sessions, Ops
- Layout: fixed top nav (72px) + collapsible side nav (250px) + main content area
- All styling in single `global.css` (~1200 lines)
- No component library, no Tailwind, no design tokens system

## Design Goals
1. **Visual identity overhaul** — Move away from generic glassmorphism toward a distinctive, memorable design language
2. **Information density** — This is a knowledge management + RAG chat tool for power users; prioritize scan-ability over whitespace
3. **Chat experience** — The ChatPage is the core product surface; it should feel like a premium AI chat interface (think Claude.ai, ChatGPT quality)
4. **Dark mode support** — Add dark/light theme toggle
5. **Better visual hierarchy** — Current pages have flat hierarchy; cards all look the same
6. **Micro-interactions** — Subtle but meaningful animations for state changes (loading, success, error)
7. **Responsive refinement** — Current breakpoints work but mobile chat experience needs polish

## Constraints
- Keep React + TypeScript + Vite stack
- Keep Zustand + React Query + react-router-dom
- Keep all existing page components and their logic — redesign styling and layout ONLY
- Must support Chinese text (keep Noto Sans SC / PingFang SC font stack)
- No new npm dependencies for styling (no Tailwind, no Chakra, no MUI)
- Pure CSS or CSS-in-JS via style objects only
- All 6 pages must be redesigned

## Pages to Redesign

### LoginPage
- Simple auth form (username/password)
- Current: centered card with glassmorphism
- Goal: More dramatic, branded landing experience

### KnowledgeBasePage
- Left: create KB form, Right: KB list with search
- CRUD for knowledge bases
- Goal: Dashboard feel, better data cards

### DocumentsPage
- Upload + index docs, list docs with preview modal
- Two-panel layout
- Goal: File manager aesthetic, drag-drop zone

### ChatPage
- Full conversation view with citations, citation modal
- Most important page — needs to feel like a premium chat product
- Goal: Claude.ai / ChatGPT quality chat bubbles, better citation UX

### SessionsPage
- Left: session list, Right: message viewer
- Goal: Email client / Slack sidebar feel

### OpsPage
- Health check dashboard with polling
- Goal: Monitoring dashboard feel, status indicators

## Color Direction
- Move toward a warmer, more sophisticated palette
- Consider: deep navy/charcoal backgrounds for dark mode, warm whites for light mode
- Accent: violet/indigo or emerald — avoid generic "SaaS blue"
- Better semantic colors for states (success, warning, error, info)

## Typography
- Keep Inter for Latin, keep CJK fallbacks
- Improve type scale — current sizes are inconsistent
- Better weight usage for hierarchy

## Technical Requirements
- CSS custom properties for theming (light/dark)
- Organized CSS architecture (not one giant file)
- Smooth theme transitions
- Print-friendly basics
