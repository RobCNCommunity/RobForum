# Roblox Community Design System

## Product Character

- Quiet, work-focused social community UI inspired by familiar timeline patterns.
- Content and repeated actions take priority over decorative marketing layouts.
- Light and dark themes share the same semantic tokens and interaction model.

## Tokens

- Primary action: `--primary` (`#1d9bf0`) with `--primary-hover` (`#1a8cd8`).
- Surfaces: `--rf-bg`, `--rf-bg-subtle`, and `--rf-bg-hover`.
- Text: `--rf-text`, `--rf-muted`, and `--rf-faint`.
- Borders: `--rf-line`; danger: `--rf-danger`; success: `--rf-success`.
- Syntax highlighting: `--rf-code-comment`, `--rf-code-keyword`, `--rf-code-string`, `--rf-code-title`, `--rf-code-number`, `--rf-code-variable`, `--rf-code-tag`, and `--rf-code-meta` provide paired light/dark code-token colors without changing the code-block surface recipe.
- Typography: MiSans is the primary face for both `--rf-font` and `--rf-font-display`, with system CJK fonts as fallbacks.
- Radius: `--rf-radius` is 8px for framed tools; `--rf-pill` is reserved for pills and compact actions.
- Touch targets: interactive controls use at least `--rf-touch` (44px) where space permits.
- Motion: `--ease` for standard transitions and spring-like cubic Bezier curves for direct manipulation.
- Theme preference: first visit follows the operating system; an explicit light or dark choice persists locally and is applied before the app renders to prevent a theme flash.

## Layout

- Desktop uses a fixed left rail, a 600px feed column, and an optional 350px right rail.
- Main content is divided by subtle 1px borders instead of floating section cards.
- Mobile uses a sticky header, bottom navigation, safe-area padding, and purpose-built compact composers. Focus routes such as post detail and profile editing own their navigation and omit the global bottom bar and publish action.
- Fixed-format media, toolbars, and action rows keep stable dimensions to prevent layout shift.

## Components

- Use `AppIcon` for recognizable actions; icon-only controls require accessible labels and tooltips where needed.
- The theme control is an icon-only sun/moon toggle in the desktop rail, mobile drawer, and public authentication shell. Its accessible label always describes the next theme.
- Use buttons for actions and links only for navigation. Never nest action buttons inside a route link.
- Repeated content may use compact rows or cards; page sections remain unframed.
- Feed post action rows expose comment, repost, quote, like, and share as stable icon controls. Comment expands the shared compact composer directly beneath the post; quote opens the post composer with visible source context.
- Post body inputs use an edit/preview segmented control. Published Markdown is rendered through the shared sanitized content component; feed and management surfaces use its compact variant.
- Mermaid diagrams and KaTeX formulas reuse the Markdown content surface. Diagrams remain horizontally navigable at narrow widths, formulas may scroll instead of shrinking below legibility, and both inherit the active light/dark semantic tokens.
- Inputs use the shared surface, border, focus ring, and semantic status colors from `styles.css`.

## Content Rules

- Optional post titles are omitted completely; the body becomes the primary text.
- Post bodies support GitHub-flavored Markdown, `$...$` inline LaTeX, `$$...$$` block LaTeX, Mermaid fenced diagrams, and explicitly labeled fenced-code highlighting for common web, backend, scripting, configuration, and database languages. Mermaid is loaded only when a non-compact surface contains a diagram; compact surfaces use readable plain-text fallbacks. Unknown code labels fall back to escaped plain text; unsafe HTML, unsafe SVG, and interactive elements are removed before rendering; external links open in a separate tab with no opener access.
- Tags are compact metadata, not a replacement for board navigation.
- Images and videos show the real uploaded media with predictable aspect constraints.
- Dense controls must remain readable in Simplified Chinese at 320px and wider.
