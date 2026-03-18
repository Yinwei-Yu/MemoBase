# Component Guidelines

> Component design standards for React frontend.

---

## Component Structure

Use function components only.

Recommended file layout (for non-trivial components):
- `ComponentName.tsx`
- `ComponentName.module.css` (or colocated style file)
- `ComponentName.test.tsx`
- `index.ts`

Component responsibilities:
- Present data.
- Emit events.
- Avoid direct data fetching and heavy business logic.

---

## Props Conventions

- Define explicit `Props` types/interfaces per component.
- Prefer narrow props over passing whole objects.
- Use callbacks for actions (`onSubmit`, `onRetry`).
- Avoid boolean explosion; replace with discriminated union for multi-mode behavior.

Example pattern:

```ts
export type ChatAnswerCardProps = {
  answer: string;
  citations: Citation[];
  isStreaming: boolean;
  onOpenCitation: (citationId: string) => void;
};
```

---

## Composition and Reuse

- Prefer composition over inheritance.
- Extract reusable UI as shared components only after second clear reuse.
- Keep feature-specific variants inside feature module until stable.

---

## Styling Patterns

- Default: CSS Modules for component-scoped styles.
- Global styles only in `src/styles`.
- Use design tokens via CSS variables (spacing, colors, typography).
- Inline styles allowed only for runtime-calculated values.

---

## Accessibility

Release-blocking requirements:
- Semantic HTML first (`button`, `label`, `main`, `nav`).
- Form controls must have labels and error descriptions.
- Keyboard navigation must work for all interactive controls.
- Focus state must be visible.
- Images/icons need meaningful `alt` or `aria-hidden` when decorative.

---

## Common Mistakes (Forbidden)

- Fetching data directly in presentational components.
- Using `div` as button without keyboard support.
- Hiding errors and showing only console logs.
- Unbounded props drilling instead of composing hooks/store.
