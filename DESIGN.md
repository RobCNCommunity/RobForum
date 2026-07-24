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
- Inputs use the shared surface, border, focus ring, and semantic status colors from `styles.css`.

## Content Rules

- Optional post titles are omitted completely; the body becomes the primary text.
- Tags are compact metadata, not a replacement for board navigation.
- Images and videos show the real uploaded media with predictable aspect constraints.
- Dense controls must remain readable in Simplified Chinese at 320px and wider.
