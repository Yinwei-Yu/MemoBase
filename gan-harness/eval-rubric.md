# Design Evaluation Rubric — KnowledgeAI UI Redesign

## Scoring: 1-10 per dimension

---

### Design Quality (weight: 0.35)
Does this look like a polished, professional product?

- **Color palette**: Cohesive, intentional, not generic. Dark/light mode both work.
- **Typography**: Clear hierarchy (h1 > h2 > body > caption), consistent scale, good CJK rendering.
- **Spacing**: Rhythmic, consistent padding/margins. No awkward gaps or cramped areas.
- **Visual hierarchy**: User knows where to look first. Important things stand out.
- **Consistency**: Same patterns used across all 6 pages. Buttons, inputs, cards feel unified.
- **Contrast / Accessibility**: Text readable on all backgrounds. WCAG AA minimum.

Score guide:
- 1-3: Looks like a Bootstrap template, no personality
- 4-6: Clean but forgettable, "SaaS default" feel
- 7-8: Distinctive, would impress in a product demo
- 9-10: Award-worthy, could be on Dribbble/Behance front page

---

### Originality (weight: 0.30)
Does this push creative boundaries or take interesting design risks?

- **Layout innovation**: Does it use layout in an unexpected way? (Not just "cards in a grid")
- **Visual language**: Does it have a unique design DNA? Custom shapes, patterns, or motifs?
- **Interaction design**: Are there creative micro-interactions, transitions, or hover states?
- **Chat UI innovation**: Does the chat page feel different from generic chat interfaces?
- **Theming approach**: Is the dark/light mode switch done in an interesting way?
- **"Would I remember this?"**: After closing the app, would the design stick in your mind?

Score guide:
- 1-3: Standard layout, nothing unexpected
- 4-6: Some nice touches but mostly conventional
- 7-8: Several creative leaps that feel intentional, not gimmicky
- 9-10: Genuinely novel approach, could inspire other designers

---

### Craft (weight: 0.25)
How refined is the execution?

- **Animation quality**: Transitions feel smooth, not jarring. Appropriate easing curves.
- **Detail work**: Border-radius consistency, shadow depth logic, focus states, scrollbar styling.
- **Responsive design**: All breakpoints work well. Mobile chat is usable.
- **Edge cases handled**: Empty states, loading states, error states all look intentional.
- **CSS architecture**: Organized, not one giant file. Good use of custom properties.
- **Performance awareness**: No gratuitous blur filters on every element. Sensible use of backdrop-filter.

Score guide:
- 1-3: Rough, inconsistent spacing, broken layouts
- 4-6: Works but has visible rough edges
- 7-8: Feels production-ready, minor details polished
- 9-10: Every pixel intentional, nothing feels off

---

### Functionality (weight: 0.10)
Does the design support the product's actual use?

- **Information density**: Power users can scan and find things quickly
- **Chat UX**: Easy to read conversations, citations are accessible, composer is ergonomic
- **Navigation clarity**: User always knows where they are and how to get elsewhere
- **Form usability**: Create KB, upload docs, login — all flows are clear
- **Status communication**: Loading, success, error states communicate clearly

Score guide:
- 1-3: Design actively hinders usability
- 4-6: Usable but could be more efficient
- 7-8: Feels natural, supports the workflow well
- 9-10: Design makes the product feel more capable than it is

---

## Overall Score Calculation
```
overall = (design_quality * 0.35) + (originality * 0.30) + (craft * 0.25) + (functionality * 0.10)
```

## Pass Threshold
- **7.5** weighted score to pass
- Any single dimension below 5 triggers a mandatory revision note
- Originality below 6 is a soft warning — "play it safer than needed"
